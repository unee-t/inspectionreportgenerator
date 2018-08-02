package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"html/template"

	"github.com/apex/log"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
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
}

func main() {
	addr := ":" + os.Getenv("PORT")
	app := mux.NewRouter()

	app.PathPrefix("/templates").Handler(http.FileServer(http.Dir(".")))
	app.HandleFunc("/", handleIndex).Methods("GET")
	app.HandleFunc("/htmlgen", handlePost).Methods("POST")
	app.HandleFunc("/pdfgen", handlePdfgen).Methods("GET")

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
	t.ExecuteTemplate(w, "index.html", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
		"Stage":          os.Getenv("UP_STAGE"),
	})
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

	t := template.Must(template.New("").ParseGlob("templates/signoff.html"))
	var b bytes.Buffer
	t.ExecuteTemplate(io.Writer(&b), "signoff.html", signoff)

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

func pdfgen(url string) (pdfurl string, err error) {

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

	payload := strings.NewReader(fmt.Sprintf("{\n  \"url\": \"%s\",\n  \"css\": \"h2 { page-break-before: always; page-break-after: avoid; } h3 { page-break-after: avoid } figure, ul, ol { page-break-inside: avoid } dt-appendix, dt-appendix h3 { page-break-before: always }\",\n  \"screen\": false,\n  \"scale\": 1,\n  \"displayHeaderFooter\": false,\n  \"printBackground\": false,\n  \"landscape\": false,\n  \"pageRanges\": \"\",\n  \"format\": \"Letter\",\n  \"margin\": {\n  \t\"top\": \"24px\",\n  \t\"right\": \"16px\",\n  \t\"bottom\": \"24px\",\n  \t\"left\": \"16px\"\n  }\n}", url))

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

	// fmt.Println(res)
	// fmt.Println(string(body))

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

	return filename, err
}

func handlePdfgen(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "Missing URL", 400)
		return
	}
	url, err := pdfgen(url)
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
