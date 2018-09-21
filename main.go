package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"path/filepath"
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

type responseHTML struct {
	HTML string
	JSON string
}

var e env.Env

func main() {

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		log.WithError(err).Fatal("setting up credentials")
		return
	}
	cfg.Region = endpoints.ApSoutheast1RegionID
	e, err = env.New(cfg)
	if err != nil {
		log.WithError(err).Warn("error getting unee-t env")
		return
	}

	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	CSRF := csrf.Protect([]byte("32-byte-long-auth-key-yeah"), csrf.Secure(false))
	if os.Getenv("UP_STAGE") != "" {
		CSRF = csrf.Protect([]byte("32-byte-long-auth-key-yeah"), csrf.Secure(true))
	}

	app.PathPrefix("/templates").Handler(http.FileServer(http.Dir(".")))
	app.HandleFunc("/", env.Towr(CSRF(http.HandlerFunc(handleIndex)))).Methods("GET")
	app.HandleFunc("/htmlgen", env.Towr(CSRF(http.HandlerFunc(handlePost)))).Methods("POST")
	app.HandleFunc("/jsonhtmlgen", env.Towr(CSRF(http.HandlerFunc(handleJSON)))).Methods("POST")
	app.HandleFunc("/pdfgen", handlePDFgen).Methods("GET")
	app.HandleFunc("/", env.Towr(env.Protect(http.HandlerFunc(handleJSON), e.GetSecret("API_ACCESS_TOKEN"))))

	if err := http.ListenAndServe(addr, app); err != nil {
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
		log.WithError(err).Fatal("bad JSON")
		http.Error(w, "JSON does not conform to https://github.com/unee-t/wetsignaturetopdfprototype/blob/master/structs.go", http.StatusBadRequest)
		return
	}
	log.Infof("%+v", ir)

	output, err := genHTML(ir)
	if err != nil {
		log.WithError(err).Fatal("genHTML from handleJSON")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response.JSON(w, output)
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
			Images: []string{
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_7126.jpg",
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_7126.jpg",
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
				"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_7126.jpg",
			},
			Cases: []Case{{
				Title: "Cracks on Ceiling",
				Images: []string{
					"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/inspection_report.jpg",
				},
				Category: "Reference",
				Status:   "Confirmed",
				Details:  "Worse over time and rain is sometimes seen to be leaking when it rains.",
			}},
			Inventory: []Item{{
				Name:        "Ikea Ivar Shelf",
				Images:      []string{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/images.jpg"},
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
							Images:   []string{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_9411.jpg", "http://res.cloudinary.com/unee-t-staging/image/upload/e_cartoonify/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_9411.jpg"},
							Category: "Repair",
							Status:   "Confirmed",
							Details:  "Lights are unable to turn on after change the light bulb",
						},
						{
							Title:    "Floor stain and the mould seems to smell",
							Images:   []string{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/wood_floor_stain.jpg"},
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
					Images:      []string{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry.jpg"},
					Cases:       nil,
					Inventory: []Item{
						{
							Name:        "LG Electronics fridge",
							Images:      []string{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry_fridge.jpg"},
							Description: "1 in acceptable working condition",
						},
						{
							Name:        "Solid Wood long table",
							Images:      []string{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry_02.jpg"},
							Description: "1 in very bad condition. Table is baldy chipped and edges are wearing out.",
						},
						{
							Name:        "Pantry cabinet",
							Images:      []string{"http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/pantry_microwave.jpg"},
							Description: "1 in good condition. Well maintained.",
						},
						{
							Name:        "Bekant chairs",
							Images:      []string{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0522.jpg", "https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0519.jpg"},
							Description: "12 in mint condition.",
						},
						{
							Name:        "More chairs",
							Images:      []string{"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0522.jpg", "https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0519.jpg"},
							Description: "12 in mint condition.",
						},
						{
							Name: "So many more chairs",
							Images: []string{
								"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0522.jpg",
								"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0519.jpg",
								"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0519.jpg",
								"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0522.jpg",
								"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0522.jpg",
								"https://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_500,w_500/v1534218648/Unee-T%20inspection%20report%20-%20placeholder%20images/IMG_0519.jpg"},
							Description: "6 in mint condition.",
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

	output, err := genHTML(signoff)
	if err != nil {
		log.WithError(err).Fatal("failed to decode form")
		http.Error(w, err.Error(), 500)
		return
	}

	response.JSON(w, output)

}

func pdfcoolgen(url string) (pdfurl string, err error) {

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		log.WithError(err).Fatal("setting up credentials")
		return
	}
	cfg.Region = endpoints.ApSoutheast1RegionID

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
		Bucket:      aws.String(e.Bucket()),
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

	return fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/%s", e.Bucket(), filename), err
}

func handlePDFgen(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")

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

	if u.Host != "s3-ap-southeast-1.amazonaws.com" {
		http.Error(w, "Source must be from our S3 region", 400)
		return
	}
	if !strings.HasPrefix(u.Path, fmt.Sprintf("/%s/", e.Bucket())) {
		http.Error(w, "Source must be from our S3 path", 400)
		return
	}

	url, err = pdfcoolgen(url)
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
		Bucket:      aws.String(e.Bucket()),
		Body:        bytes.NewReader(dataJSON),
		Key:         aws.String(jsonfilename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("application/json; charset=UTF-8"),
	}

	req := svc.PutObjectRequest(putparams)
	_, err = req.Send()

	return fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/%s", e.Bucket(), jsonfilename), err
}

// CloudinaryTransform takes a Cloudinary URL and outputs the transformations we want to see
func CloudinaryTransform(url string, transforms string) (transformedURL string, err error) {
	// https://res.cloudinary.com/<cloud_name>/<resource_type>/<type>/<version>/<transformations>/<public_id>.<format>
	// Optional values: resource_type, type, version, transformations, format
	uParsed, err := neturl.ParseRequestURI(url)
	if err != nil {
		return "", err
	}
	// log.Infof("%+v\n", *uParsed)
	if uParsed.Host != "res.cloudinary.com" {
		return "", fmt.Errorf("%s is not a cloudinary host", uParsed.Host)
	}
	uParsed.Scheme = "https"
	s := strings.Split(uParsed.Path, "/")
	s = append(s[:2], append([]string{transforms}, s[2:]...)...)
	// log.Infof("%+v", s)
	uParsed.Path = strings.Join(append(s[0:3], s[len(s)-2:]...), "/")
	// log.Infof("Right? %+v", uParsed.Path)
	return uParsed.String(), nil
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func genHTML(ir InspectionReport) (output responseHTML, err error) {

	randomString, err := randomHex(2)
	if err != nil {
		return
	}

	var filename = fmt.Sprintf("%s-%s", ir.ID, randomString)

	ir.Report.Images = updateImages(ir.Report.Images)
	for item := 0; item < len(ir.Report.Inventory); item++ {
		ir.Report.Inventory[item].Images = updateImages(ir.Report.Inventory[item].Images)
	}
	for room := 0; room < len(ir.Report.Rooms); room++ {
		ir.Report.Rooms[room].Images = updateImages(ir.Report.Rooms[room].Images)

		for item := 0; item < len(ir.Report.Rooms[room].Inventory); item++ {
			ir.Report.Rooms[room].Inventory[item].Images = updateImages(ir.Report.Rooms[room].Inventory[item].Images)
		}

		for c := 0; c < len(ir.Report.Rooms[room].Cases); c++ {
			ir.Report.Rooms[room].Cases[c].Images = updateImages(ir.Report.Rooms[room].Cases[c].Images)
		}

	}
	for c := 0; c < len(ir.Report.Cases); c++ {
		ir.Report.Cases[c].Images = updateImages(ir.Report.Cases[c].Images)
	}

	var t *template.Template
	var b bytes.Buffer

	if ir.Template == "" {
		// Use default template
		t, err = template.New("").Funcs(template.FuncMap{
			"formatDate": func(d time.Time) string { return d.Format("2 Jan 2006") },
			"increment":  func(i int) int { return i + 1 },
			"domain":     func() string { return e.Udomain("case") },
		}).ParseFiles("templates/signoff.html")

		if err != nil {
			return output, err
		}
		err = t.ExecuteTemplate(io.Writer(&b), "signoff.html", ir)
	} else {
		resp, err := http.Get(ir.Template)
		if err != nil {
			return output, err
		}
		defer resp.Body.Close()
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return output, err
		}
		tmpl, err := template.New("").Funcs(template.FuncMap{
			"formatDate": func(d time.Time) string { return d.Format("2 Jan 2006") },
			"increment":  func(i int) int { return i + 1 },
			"domain":     func() string { return e.Udomain("case") },
		}).Parse(string(contents))
		err = tmpl.Execute(io.Writer(&b), ir)
	}

	if err != nil {
		return output, err
	}

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		return output, err
	}
	svc := s3.New(cfg)

	dumpurl, err := dump(svc, filename, ir)
	if err != nil {
		return output, err
	}
	log.Infof("dumpurl %s", dumpurl)

	htmlfilename := time.Now().Format("2006-01-02") + "/" + filename + ".html"
	putparams := &s3.PutObjectInput{
		Bucket:      aws.String(e.Bucket()),
		Body:        bytes.NewReader(b.Bytes()),
		Key:         aws.String(htmlfilename),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("text/html; charset=UTF-8"),
	}

	req := svc.PutObjectRequest(putparams)
	_, err = req.Send()

	if err != nil {
		return output, err
	}

	return responseHTML{
		HTML: fmt.Sprintf("https://s3-ap-southeast-1.amazonaws.com/%s/%s", e.Bucket(), htmlfilename),
		JSON: dumpurl,
	}, err

}

func updateImages(images []string) []string {
	for i := 0; i < len(images); i++ {
		images[i], _ = CloudinaryTransform(images[i], "c_fill,g_auto,h_500,w_500")
		log.Infof("Updated: %s", images[i])
	}
	return images
}
