package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"html/template"

	"github.com/apex/log"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/csrf"
	"github.com/gorilla/pat"
	"github.com/gorilla/schema"
	"github.com/tj/go/http/response"
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
	app := pat.New()

	app.PathPrefix("/templates").Handler(http.FileServer(http.Dir(".")))
	app.Get("/", handleIndex)
	app.Post("/pdfgen", handlePost)

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

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	t.ExecuteTemplate(io.Writer(&b), "signoff.html", signoff)

	cfg, err := external.LoadDefaultAWSConfig(external.WithSharedConfigProfile("uneet-dev"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), 500)
		return
	}

	log.Infof("Wrote to s3: https://media.dev.unee-t.com/%s", filename)

	response.JSON(w, struct {
		HTML string
	}{
		fmt.Sprintf("https://media.dev.unee-t.com/%s", filename),
	})

}
