package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"html/template"
)

// https://github.com/validator/validator/wiki/Output-%C2%BB-JSON
type ValidationResults struct {
	Messages []struct {
		Type         string `json:"type"`
		LastLine     int    `json:"lastLine"`
		LastColumn   int    `json:"lastColumn"`
		Message      string `json:"message"`
		Extract      string `json:"extract"`
		HiliteStart  int    `json:"hiliteStart"`
		HiliteLength int    `json:"hiliteLength"`
		FirstColumn  int    `json:"firstColumn,omitempty"`
		SubType      string `json:"subType,omitempty"`
	} `json:"messages"`
}

func TestCloudinaryTransform(t *testing.T) {
	type args struct {
		url        string
		transforms string
	}
	tests := []struct {
		name               string
		args               args
		wantTransformedURL string
		wantErr            bool
	}{
		{
			name: "Basic",
			args: args{
				url:        "https://res.cloudinary.com/unee-t-dev/image/upload/attachments/zd7reouq5i85pmsqrh70.jpg",
				transforms: "c_fill,g_auto,h_500,w_500",
			},
			wantTransformedURL: "https://res.cloudinary.com/unee-t-dev/c_fill,g_auto,h_500,w_500/attachments/zd7reouq5i85pmsqrh70.jpg",
			wantErr:            false,
		},
		{
			name: "Not a URL",
			args: args{
				url: "Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
			},
			wantErr: true,
		},
		{
			name: "Already transformed",
			args: args{
				url:        "http://res.cloudinary.com/unee-t-staging/image/upload/c_fill,g_auto,h_150,w_150/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
				transforms: "c_fill,g_auto,h_500,w_500",
			},
			wantTransformedURL: "https://res.cloudinary.com/unee-t-staging/c_fill,g_auto,h_500,w_500/Unee-T%20inspection%20report%20-%20placeholder%20images/table_succulent.jpg",
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTransformedURL, err := CloudinaryTransform(tt.args.url, tt.args.transforms)
			if (err != nil) != tt.wantErr {
				t.Errorf("CloudinaryTransform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotTransformedURL != tt.wantTransformedURL {
				t.Errorf("CloudinaryTransform() = %v, want %v", gotTransformedURL, tt.wantTransformedURL)
			}
		})
	}
}

func TestSignoffIsValid(t *testing.T) {
	var b bytes.Buffer
	tmpl, err := template.New("signoff").Funcs(template.FuncMap{
		"prettyDate": func(d time.Time) string { return d.Format("2 Jan 2006") },
		"ymdDate":    func(d time.Time) string { return d.Format("2006-01-02") },
		"increment":  func(i int) int { return i + 1 },
		"domain":     func(s string) string { return fmt.Sprintf("%s.example.com", s) },
	}).ParseFiles("templates/signoff.html")
	if err != nil {
		t.Errorf("signoff.html failed to parse, error = %v", err)
		return
	}

	ir := New()

	// jsonFile, err := os.Open("tests/test.json")
	// if err != nil {
	// 	t.Fatalf("tests/test.json failed to open, error = %v", err)
	// }
	// defer jsonFile.Close()

	// byteValue, _ := ioutil.ReadAll(jsonFile)
	// json.Unmarshal(byteValue, &ir)

	err = tmpl.ExecuteTemplate(io.Writer(&b), "signoff.html", ir)
	if err != nil {
		t.Errorf("failed to execute, error = %v", err)
		return
	}

	// err = ioutil.WriteFile("testme.html", b.Bytes(), 0644)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	req, err := http.NewRequest("POST", "https://validator.w3.org/nu/?out=json", bytes.NewReader(b.Bytes()))
	req.Header.Set("Content-Type", "text/html")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	t.Log(resp.Status)
	// body, _ := ioutil.ReadAll(resp.Body)
	// err = ioutil.WriteFile("v.json", body, 0644)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	var v ValidationResults
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&v)

	for _, m := range v.Messages {
		if strings.Contains(m.Message, "flow") { // Ignore Print CSS: “flow”: Property “flow” doesn't exist.
			continue
		}
		if strings.Contains(m.Message, "size") { // Ignore Print CSS: “size”: Property “size” doesn't exist.
			continue
		}
		if strings.Contains(m.Message, "top-left") { // Ignore “top-left”: Parse Error
			continue
		}
		if m.Type == "error" {
			t.Errorf("signoff.html line %d: %s", m.LastLine, m.Message)
		}
	}

}
