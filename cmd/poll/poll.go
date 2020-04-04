package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/FabianWe/gopolls"
	"github.com/markbates/pkger"
	"github.com/shurcooL/httpfs/html/vfstemplate"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

const version = "0.0.1"

var currencyHandler = gopolls.SimpleEuroHandler{}

type mainContext struct {
	Voters         []*gopolls.Voter
	PollCollection *gopolls.PollSkeletonCollection
	// in case collection was loaded from a file this value is set to this path
	CollectionSourcePath string
}

type renderContext struct {
	*mainContext
}

type appHandler interface {
	Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) (int, error)
}

func toHandleFunc(h appHandler, context *mainContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buff bytes.Buffer
		statusCode, err := h.Handle(context, &buff, r)
		if err != nil {
			log.Println("Unable to write to http response", err)
			http.Error(w, "Internal error", statusCode)
			return
		}
		content, contentErr := ioutil.ReadAll(&buff)
		if contentErr != nil {
			log.Println("error:", contentErr)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		_, writeErr := w.Write(content)
		if writeErr != nil {
			log.Println("Unable to write to http response", writeErr)
			return
		}
	}
}

func baseTemplates() *template.Template {
	return template.Must(vfstemplate.ParseFiles(pkger.Dir("/cmd/poll/templates/"), nil,"base.html"))
}

type mainHandler struct {
	template *template.Template
}

//func newMainHandler() *mainHandler {
//	t := readPkgTemplate("/cmd/poll/templates/main.html")
//	return &mainHandler{t}
//}

func newMainHandler(base *template.Template) *mainHandler {
	t := template.Must(vfstemplate.ParseFiles(pkger.Dir("/cmd/poll/templates/"), template.Must(base.Clone()), "index.html"))
	return &mainHandler{t}
}

func (h *mainHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) (int, error) {
	data := &renderContext{context}
	templateErr := h.template.Execute(buff, data)
	if templateErr != nil {
		return http.StatusInternalServerError, templateErr
	}

	return http.StatusOK, nil
}



func main() {
	pkger.Include("/cmd/poll/templates")
	pkger.Include("/cmd/poll/static")

	base := baseTemplates()

	context := mainContext{}
	context.PollCollection = gopolls.NewPollSkeletonCollection("foo")
	mainH := newMainHandler(base)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(pkger.Dir("/cmd/poll/static"))))
	http.HandleFunc("/", toHandleFunc(mainH, &context))
	addr := "localhost:8080"
	log.Printf("Running server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func readVoters(file string) ([]*gopolls.Voter, error) {
	// open Voters
	votersFile, votersFileErr := os.Open(file)
	if votersFileErr != nil {
		return nil, votersFileErr
	}
	defer votersFile.Close()
	voters, votersErr := gopolls.ParseVoters(votersFile)
	if votersErr != nil {
		return nil, votersErr
	}
	fmt.Printf("Read %d Voters\n", len(voters))
	return voters, nil
}

func readPolls(file string) (*gopolls.PollSkeletonCollection, error) {
	pollsFile, pollsFileErr := os.Open(file)
	if pollsFileErr != nil {
		return nil, pollsFileErr
	}
	defer pollsFile.Close()
	collection, collectionErr := gopolls.ParseCollectionSkeletons(pollsFile, currencyHandler)
	if collectionErr != nil {
		return nil, collectionErr
	}
	fmt.Printf("Parsed polls for \"%s\":\n", collection.Title)
	fmt.Printf("  # Groups = %d\n  # Polls = %d\n",
		collection.NumGroups(), collection.NumSkeletons())
	return collection, nil
}

func runMain() error {
	fmt.Printf("gopolls version %s started\n", version)

	flags := flag.NewFlagSet("gopolls", flag.ExitOnError)
	var votersFilePath string
	flags.StringVar(&votersFilePath, "Voters", "", "Filepath to the Voters file")
	var pollsFilePath string
	flags.StringVar(&pollsFilePath, "polls", "", "Path to the polls file")
	flags.Parse(os.Args[1:])

	return nil
}
