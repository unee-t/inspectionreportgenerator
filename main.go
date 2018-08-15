package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"html/template"

	"github.com/apex/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/tj/go/http/response"
	"github.com/unee-t/env"
)

// Signature holds the wet signature
type Signature struct {
	Name    string // Who
	Role    string
	DataURI template.URL // What: Graphic signature
}

// Case summarises the cases
type Case struct {
	Title    string
	Images   []template.URL
	Category string
	Status   string
	Details  string
}

// Information pertaining to the Unit
type Information struct { // How to see object in Mongo ?
	Name        string
	Type        string
	Address     string
	Postcode    string
	City        string
	State       string
	Country     string
	Description string
}

// Unit is actually a Product in Bugzilla
type Unit struct {
	Information Information // Stored in Mongo
}

// Item is part of an Inventory
type Item struct {
	Name        string
	Images      []template.URL
	Description string
	// Not needed right now
	// Cases       []Case // TODO: not sure what this looks like in the published report
}

// Report for the Unit and rooms of the unit
type Report struct {
	Name        string // Handover of unit – 20 Maple Avenue, Unit 01-02
	Creator     string // email from bugzilla
	Description string
	Images      []template.URL
	Cases       []Case
	Inventory   []Item
	Rooms       []Room
	Comments    string
}

// Room each can have issues (cases) and an inventory
type Room struct {
	Name        string
	Description string
	Images      []template.URL
	Cases       []Case
	Inventory   []Item
}

// InspectionReport is the top level structure that holds a report
type InspectionReport struct {
	ID         string
	Date       time.Time
	Signatures []Signature
	Unit       Unit
	Report     Report
	Template   string
}

func main() {
	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	app.PathPrefix("/templates").Handler(http.FileServer(http.Dir(".")))
	app.HandleFunc("/", handleIndex).Methods("GET")
	app.HandleFunc("/htmlgen", handlePost).Methods("POST")
	app.HandleFunc("/jsonhtmlgen", handleJSON).Methods("POST")
	app.HandleFunc("/pdfgen", handlePDFgen).Methods("GET")

	var options []csrf.Option

	// If developing locally
	options = append(options, csrf.Secure(false))

	if err := http.ListenAndServe(addr,
		csrf.Protect([]byte("32-byte-long-auth-key"), options...)(app)); err != nil {
		log.WithError(err).Fatal("error listening")
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {

	if os.Getenv("UP_STAGE") != "production" {
		w.Header().Set("X-Robots-Tag", "none")
	}

	t := template.Must(template.New("").ParseFiles("templates/index.html"))
	err := t.ExecuteTemplate(w, "index.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
		"Stage":          os.Getenv("UP_STAGE"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func handleJSON(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var ir InspectionReport
	err := decoder.Decode(&ir)
	if err != nil {
		panic(err)
	}
	log.Infof("%+v", ir)

	var filename = ""

	for _, v := range ir.Signatures {
		filename += strings.ToLower(v.Name)
	}

	reg, _ := regexp.Compile("[^a-z]+")
	filename = reg.ReplaceAllString(filename, "") + ".html"

	var t *template.Template
	var b bytes.Buffer

	if ir.Template == "" {
		// Use default template
		t, err = template.New("").Funcs(template.FuncMap{
			"formatDate": func(d time.Time) string { return d.Format("2 Jan 2006") },
			"increment":  func(i int) int { return i + 1 },
		}).ParseFiles("templates/signoff.html")

		if err != nil {
			log.WithError(err).Fatal("failed to parse signoff.html")
			http.Error(w, err.Error(), 500)
			return
		}
		err = t.ExecuteTemplate(io.Writer(&b), "signoff.html", ir)
	} else {
		resp, err := http.Get(ir.Template)
		if err != nil {
			log.WithError(err).Fatalf("failed to fetch %s", ir.Template)
			http.Error(w, err.Error(), 500)
			return
		}
		defer resp.Body.Close()
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Fatalf("failed to parse %s", ir.Template)
			http.Error(w, err.Error(), 500)
			return
		}
		tmpl, err := template.New("").Funcs(template.FuncMap{
			"formatDate": func(d time.Time) string { return d.Format("2 Jan 2006") },
			"increment":  func(i int) int { return i + 1 },
		}).Parse(string(contents))
		err = tmpl.Execute(io.Writer(&b), ir)
	}

	if err != nil {
		log.WithError(err).Fatal("failed to execute template")
		http.Error(w, err.Error(), 500)
		return
	}

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		log.WithError(err).Fatal("failed to get config")
		http.Error(w, err.Error(), 500)
		return
	}
	svc := s3.New(cfg)

	filename = time.Now().Format("2006-01-02") + "/" + filename
	putparams := &s3.PutObjectInput{
		Bucket:      aws.String("dev-media-unee-t"),
		Body:        bytes.NewReader(b.Bytes()),
		Key:         aws.String(filename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("text/html; charset=UTF-8"),
	}

	req := svc.PutObjectRequest(putparams)
	_, err = req.Send()

	if err != nil {
		log.WithError(err).Fatal("failed to put")
		http.Error(w, err.Error(), 500)
		return
	}

	response.JSON(w, struct {
		HTML string
	}{
		fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/dev-media-unee-t/%s", filename),
	})

}

func handlePost(w http.ResponseWriter, r *http.Request) {

	err := r.ParseMultipartForm(0)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	signoff := InspectionReport{
		ID:         "12345678",
		Date:       time.Now(),
		Signatures: nil,
		Unit: Unit{
			Information: Information{
				Name:        "Unit 01-02",
				Type:        "Apartment/Flat",
				Address:     "20 Maple Avenue",
				Postcode:    "90731",
				City:        "San Pedro",
				State:       "California",
				Country:     "USA",
				Description: "Blue house with a front porch. Parking is not allowed in the driveway",
			},
		},
		Report: Report{
			Name: "20 Maple Avenue, Unit 01-02",
			Images: []template.URL{
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_7126.jpg",
			},
			Cases: []Case{{
				Title: "Cracks on Ceiling",
				Images: []template.URL{
					"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/inspection_report.jpg",
				},
				Category: "Reference",
				Status:   "Confirmed",
				Details:  "Worse over time and rain is sometimes seen to be leaking when it rains.",
			}},
			Inventory: []Item{{
				Name:        "Ikea Ivar Shelf",
				Images:      []template.URL{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/images.jpg"},
				Description: "1 in acceptable condition",
			},
			},
			Rooms: []Room{
				{
					Name:        "Big Meeting Room",
					Description: "300 sqft with built-in cabinets, air-con and WiFi",
					Images:      nil,
					Cases: []Case{
						{
							Title:    "Light is not working",
							Images:   []template.URL{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_9411.jpg", "http://res.cloudinary.com/unee-t-staging/image/upload/e_cartoonify/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_9411.jpg"},
							Category: "Repair",
							Status:   "Confirmed",
							Details:  "Lights are unable to turn on after change the light bulb",
						},
						{
							Title:    "Floor stain and the mould seems to smell",
							Images:   []template.URL{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/wood_floor_stain.jpg"},
							Category: "Complex project",
							Status:   "Reopened",
							Details:  "Horrible floor statins are appearing due to moisture over time. There is a bad smell.",
						},
					},
					Inventory: nil,
				},
				{
					Name:        "Pantry",
					Description: "800 sqft, high with built-in cabinets, air-con and WiFi",
					Images:      []template.URL{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry.jpg"},
					Cases:       nil,
					Inventory: []Item{
						{
							Name:        "LG Electronics fridge",
							Images:      []template.URL{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry_fridge.jpg"},
							Description: "1 in acceptable working condition",
						},
						{
							Name:        "Solid Wood long table",
							Images:      []template.URL{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry_02.jpg"},
							Description: "1 in very bad condition. Table is baldy chipped and edges are wearing out.",
						},
						{
							Name:        "Pantry cabinet",
							Images:      []template.URL{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry_microwave.jpg"},
							Description: "1 in good condition. Well maintained.",
						},
						{
							Name:        "Bekant chairs",
							Images:      []template.URL{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0522.jpg", "https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0519.jpg"},
							Description: "12 in mint condition.",
						},
					},
				},
			},
			Comments: "A comment pertaining to the report itself.",
		},
	}

	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	err = decoder.Decode(&signoff, r.PostForm)

	if err != nil {
		log.WithError(err).Fatal("failed to decode form")
		http.Error(w, err.Error(), 500)
		return
	}

	var filename = ""

	for _, v := range signoff.Signatures {
		filename += strings.ToLower(v.Name)
	}

	reg, _ := regexp.Compile("[^a-z]+")
	filename = reg.ReplaceAllString(filename, "")

	t, err := template.New("").Funcs(template.FuncMap{
		"formatDate": func(d time.Time) string { return d.Format("2 Jan 2006") },
		"increment":  func(i int) int { return i + 1 },
	}).ParseFiles("templates/signoff.html")

	if err != nil {
		log.WithError(err).Fatal("failed to parse signoff.html")
		http.Error(w, err.Error(), 500)
		return
	}

	var b bytes.Buffer

	err = t.ExecuteTemplate(io.Writer(&b), "signoff.html", signoff)
	if err != nil {
		log.WithError(err).Fatal("failed to execute template signoff")
		http.Error(w, err.Error(), 500)
		return
	}

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		log.WithError(err).Fatal("failed to get config")
		http.Error(w, err.Error(), 500)
		return
	}
	svc := s3.New(cfg)

	dumpurl, err := dump(svc, filename, signoff)
	if err != nil {
		log.WithError(err).Fatal("failed to dump")
		http.Error(w, err.Error(), 500)
		return
	}

	htmlfilename := time.Now().Format("2006-01-02") + "/" + filename + ".html"
	putparams := &s3.PutObjectInput{
		Bucket:      aws.String("dev-media-unee-t"),
		Body:        bytes.NewReader(b.Bytes()),
		Key:         aws.String(htmlfilename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("text/html; charset=UTF-8"),
	}

	req := svc.PutObjectRequest(putparams)
	_, err = req.Send()

	if err != nil {
		log.WithError(err).Fatal("failed to put")
		http.Error(w, err.Error(), 500)
		return
	}

	response.JSON(w, struct {
		HTML string
		JSON string
	}{
		fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/dev-media-unee-t/%s", htmlfilename),
		dumpurl,
	})

}

func pdfraptorgen(url string) (pdfurl string, err error) {

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		log.WithError(err).Fatal("setting up credentials")
		return
	}
	cfg.Region = endpoints.ApSoutheast1RegionID
	e, err := env.New(cfg)
	if err != nil {
		log.WithError(err).Warn("error getting unee-t env")
		return
	}

	// https://documenter.getpostman.com/view/2810998/pdfcool/77mXfrG
	payload := new(bytes.Buffer)
	enc := json.NewEncoder(payload)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	enc.Encode(struct {
		URL  string `json:"document_url"`
		User string `json:"user_credentials"`
		Test bool   `json:"test"`
	}{
		url,
		e.GetSecret("RAPTORAPIKEY"),
		false,
	})

	// https://docraptor.com/documentation

	log.Infof("docraptor payload: %s", payload.String())

	req, err := http.NewRequest("POST", "https://docraptor.com/docs", payload)

	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		log.WithError(err).Fatal("failed to make request")
		return
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	svc := s3.New(cfg)

	basename := path.Base(url)
	filename := time.Now().Format("2006-01-02") + "/p-" + strings.TrimSuffix(basename, filepath.Ext(basename)) + ".pdf"
	putparams := &s3.PutObjectInput{
		Bucket:      aws.String("dev-media-unee-t"),
		Body:        bytes.NewReader(body),
		Key:         aws.String(filename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("application/pdf; charset=UTF-8"),
	}

	s3req := svc.PutObjectRequest(putparams)
	_, err = s3req.Send()

	if err != nil {
		log.WithError(err).Fatal("failed to put")
		return
	}

	return "https://s3-ap-southeast-1.amazonaws.com/dev-media-unee-t/" + filename, err
}

func pdfcoolgen(url string) (pdfurl string, err error) {

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		log.WithError(err).Fatal("setting up credentials")
		return
	}
	cfg.Region = endpoints.ApSoutheast1RegionID
	e, err := env.New(cfg)
	if err != nil {
		log.WithError(err).Warn("error getting unee-t env")
		return
	}

	// https://documenter.getpostman.com/view/2810998/pdfcool/77mXfrG
	payload := new(bytes.Buffer)
	enc := json.NewEncoder(payload)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	enc.Encode(struct {
		URL    string `json:"url"`
		Screen bool   `json:"screen"`
		Format string `json:"format"`
	}{
		url,
		false,
		"A4",
	})

	log.Infof("pdf.cool payload: %s", payload.String())

	req, err := http.NewRequest("POST", "https://pdf.cool/generate", payload)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+e.GetSecret("PDFCOOLTOKEN"))

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		log.WithError(err).Fatal("failed to make request")
		return
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	svc := s3.New(cfg)

	basename := path.Base(url)
	filename := time.Now().Format("2006-01-02") + "/" + strings.TrimSuffix(basename, filepath.Ext(basename)) + ".pdf"
	putparams := &s3.PutObjectInput{
		Bucket:      aws.String("dev-media-unee-t"),
		Body:        bytes.NewReader(body),
		Key:         aws.String(filename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("application/pdf; charset=UTF-8"),
	}

	s3req := svc.PutObjectRequest(putparams)
	_, err = s3req.Send()

	if err != nil {
		log.WithError(err).Fatal("failed to put")
		return
	}

	return "https://s3-ap-southeast-1.amazonaws.com/dev-media-unee-t/" + filename, err
}

func handlePDFgen(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	svc := r.URL.Query().Get("svc")

	if url == "" {
		http.Error(w, "Missing URL", 400)
		return
	}

	u, err := neturl.Parse(url)
	if err != nil {
		log.WithError(err).Fatal("not a URL")
		http.Error(w, "Missing URL", 400)
		return
	}

	if u.Host != "s3-ap-southeast-1.amazonaws.com" &&
		strings.HasPrefix(u.Path, "/dev-media-unee-t/") {
		http.Error(w, "Source must be from our S3", 400)
		return
	}

	if svc == "raptor" {
		url, err = pdfraptorgen(url)
	} else {
		url, err = pdfcoolgen(url)
	}
	if err != nil {
		log.WithError(err).Fatal("failed to generate PDF")
		http.Error(w, err.Error(), 500)
		return
	}

	response.JSON(w, struct {
		PDF string
	}{
		url,
	})
}

func dump(svc *s3.S3, filename string, data interface{}) (dumpurl string, err error) {
	dataJSON, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}

	jsonfilename := time.Now().Format("2006-01-02") + "/" + filename + ".json"
	putparams := &s3.PutObjectInput{
		Bucket:      aws.String("dev-media-unee-t"),
		Body:        bytes.NewReader(dataJSON),
		Key:         aws.String(jsonfilename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("application/json; charset=UTF-8"),
	}

	req := svc.PutObjectRequest(putparams)
	_, err = req.Send()

	return "https://s3-ap-southeast-1.amazonaws.com/dev-media-unee-t/" + jsonfilename, err
}
