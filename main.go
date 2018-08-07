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

type Signature struct {
	Name    string
	DataURI template.URL
}

type Signoff struct {
	Signatures []Signature
	Unitname   string
}

func main() {
	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	app.PathPrefix("/templates").Handler(http.FileServer(http.Dir(".")))
	app.HandleFunc("/", handleIndex).Methods("GET")
	app.HandleFunc("/htmlgen", handlePost).Methods("POST")
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

	t := template.Must(template.New("").ParseGlob("templates/*.html"))
	err := t.ExecuteTemplate(w, "index.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
		"Stage":          os.Getenv("UP_STAGE"),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func handlePost(w http.ResponseWriter, r *http.Request) {

	err := r.ParseMultipartForm(0)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	signoff := new(Signoff)
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	err = decoder.Decode(signoff, r.PostForm)

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
	filename = reg.ReplaceAllString(filename, "") + ".html"

	t, err := template.New("").ParseFiles("templates/signoff.html")
	if err != nil {
		log.WithError(err).Fatal("failed to parse signoff.html")
		http.Error(w, err.Error(), 500)
		return
	}

	var b bytes.Buffer
	signoff.Unitname = "20 Maple Avenue, Unit 01-02"
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
		Url  string `json:"document_url"`
		User string `json:"user_credentials"`
		Test bool   `json:"test"`
	}{
		url,
		e.GetSecret("RAPTORAPIKEY"),
		true,
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
	type Margin struct {
		Top    string `json:"top"`
		Right  string `json:"right"`
		Bottom string `json:"bottom"`
		Left   string `json:"left"`
	}
	enc.Encode(struct {
		Url        string `json:"url"`
		Screen     bool   `json:"screen"`
		HeaderHTML string `json:"headerHTML"`
		FooterHTML string `json:"footerHTML"`
		Format     string `json:"format"`
		Margin     Margin `json:"margin"`
	}{
		url,
		false,
		"<h1 style='float: right; text-align: right; font-size: 24px;'><a href='https://case.unee-t.com/i/123456789'>https://case.unee-t.com/i/123456789</a></h1>",
		"<small>World of Warcraft</small>",
		"A4",
		Margin{"72px", "16px", "80px", "16px"},
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
