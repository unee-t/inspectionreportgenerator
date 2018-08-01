package main

import (
	"fmt"
	"net/http"
	"os"

	"html/template"

	"github.com/apex/log"
	"github.com/gorilla/csrf"
	"github.com/gorilla/pat"
	"github.com/gorilla/schema"
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

	for _, v := range signoff.Signatures {
		fmt.Println(v.Name)
		// fmt.Println(v.DataURI)
	}

	t := template.Must(template.New("").ParseGlob("templates/signoff.html"))
	f, err := os.Create("signed.html")
	if err != nil {
		return
	}
	t.ExecuteTemplate(f, "signoff.html", signoff)
	f.Close()

}
