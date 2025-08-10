// Experimental implementation of media uploader for user-generated content (UGC).
// Media is uploaded to a unique Google Storage Bucket object named ausoceantv-ugc/username/filename.
// Using the same filename a second time overwrites the previous bucket object.
// Set GOOGLE_APPLICATION_CREDENTIALS to AusOceanTV-5dd467893726.json to run.

package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"reflect"

	"cloud.google.com/go/storage"
)

type pageData struct {
	Username string
	Msg      string
}

const bucketName = "ausoceantv-ugc"

var templates *template.Template

func main() {
	http.HandleFunc("/upload", uploadHandler)

	var err error
	templates, err = template.New("").ParseGlob("t/*.html")
	if err != nil {
		log.Fatalf("error parsing templates: %v", err)
	}

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// uploadHandler handles media uploading.
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// In AusOcean TV, we would authenticate the user here.

	pd := pageData{}

	if r.Method == "GET" {
		writeTemplate(w, r, "upload.html", &pd, "")
		return
	}

	n, err := upload(w, r)
	switch err {
	case nil:
		pd.Username = r.FormValue("username")
		writeTemplate(w, r, "upload.html", &pd, fmt.Sprintf("Uploaded %d bytes", n))

	default:
		log.Printf("upload failed: %v", err.Error())
		writeTemplate(w, r, "upload.html", &pd, err.Error())
	}
}

// upload implements the uploadHandler logic, returning the number of bytes uploaded or an error otherwise.
func upload(w http.ResponseWriter, r *http.Request) (int, error) {
	ctx := r.Context()

	username := r.FormValue("username")
	if username == "" {
		return 0, errors.New("missing username")
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		return 0, fmt.Errorf("missing file: %w", err)
	}
	log.Printf("uploading %s with %d bytes", fh.Filename, fh.Size)

	content := make([]byte, fh.Size)
	n, err := io.ReadFull(f, content)
	if err != nil {
		return 0, fmt.Errorf("error reading body: %w", err)
	}

	err = writeBucket(ctx, content, username, fh.Filename)
	if err != nil {
		return 0, fmt.Errorf("error writing media to bucket: %w", err)
	}

	log.Printf("uploaded %s with %d bytes", fh.Filename, fh.Size)
	return n, nil
}

// writeBucket writes data to a Google storage bucket named ausoceantv-ugc/username/filename.
func writeBucket(ctx context.Context, data []byte, username, filename string) error {
	clt, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}

	bkt := clt.Bucket(bucketName)
	objName := fmt.Sprintf("%s/%s", username, filename)
	writer := bkt.Object(objName).NewWriter(ctx)

	_, err = writer.Write(data)
	if err != nil {
		writer.Close()
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	log.Printf("created bucket object: gs://%s/%s", bucketName, objName)
	return nil
}

// writeTemplate writes the given template with the supplied data.
func writeTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}, msg string) {
	v := reflect.Indirect(reflect.ValueOf(data))

	p := v.FieldByName("Msg")
	if p.IsValid() {
		p.SetString(msg)
	}

	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Fatalf("ExecuteTemplate failed on %s: %v", name, err)
	}
}
