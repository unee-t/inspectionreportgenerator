package main

import (
	"net/http"
	"net/http/httputil"
	"os"

	"html/template"

	"github.com/apex/log"
	"github.com/gorilla/csrf"
	"github.com/gorilla/pat"
)

func main() {
	addr := ":" + os.Getenv("PORT")
	app := pat.New()

	app.PathPrefix("/templates").Handler(http.FileServer(http.Dir(".")))
	app.Get("/", handleIndex)
	app.Post("/pdfgen", handlePost)

	var options []csrf.Option
	// If developing locally
	// options = append(options, csrf.Secure(false))

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
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	err = r.ParseMultipartForm(0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	for key, values := range r.PostForm { // range over map
		for _, value := range values { // range over []string
			log.Infof("Key: %v Value: %v", key, value)
		}
	}

	log.Info(string(dump))
}
