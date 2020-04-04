// Copyright 2020 Fabian Wenzelmann <fabianwen@posteo.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/FabianWe/gopolls"
	"github.com/markbates/pkger"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
)

const version = "0.0.1"

var currencyHandler = gopolls.SimpleEuroHandler{}

type mainContext struct {
	Voters         []*gopolls.Voter
	PollCollection *gopolls.PollSkeletonCollection
	// in case voters were loaded from a file this value is set to the name
	VotersSourceFileName string
	// in case collection was loaded from a file this value is set to this path
	CollectionSourceFileName string
}

type renderContext struct {
	*mainContext
	AdditionalData map[string]interface{}
}

func newRenderContext(mainCtx *mainContext) *renderContext {
	return &renderContext{
		mainContext:    mainCtx,
		AdditionalData: make(map[string]interface{}),
	}
}

type handlerRes struct {
	Status      int
	Redirect    string
	ContentType string
	Err         error
}

func newHandlerRes(status int, err error) handlerRes {
	return handlerRes{
		Status:      status,
		Redirect:    "",
		ContentType: "",
		Err:         err,
	}
}

func newRedirectHandlerRes(status int, redirect string) handlerRes {
	return handlerRes{
		Status:   status,
		Redirect: redirect,
		Err:      nil,
	}
}

type appHandler interface {
	Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes
}

func toHandleFunc(h appHandler, context *mainContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handler %s called for %s\n",
			reflect.TypeOf(h), r.URL)
		var buff bytes.Buffer
		handlerRes := h.Handle(context, &buff, r)
		if handlerRes.ContentType != "" {
			w.Header().Add("Content-Type", handlerRes.ContentType)
		}
		if err := handlerRes.Err; err != nil {
			log.Println("Unable to write to http response", err)
			http.Error(w, "Internal error", handlerRes.Status)
			return
		}
		if handlerRes.Redirect != "" {
			http.Redirect(w, r, handlerRes.Redirect, handlerRes.Status)
			return
		}

		_, writeErr := io.Copy(w, &buff)
		if writeErr != nil {
			log.Println("Unable to write to http response", writeErr)
			return
		}
	}
}

func baseTemplates() *template.Template {
	funcMap := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
		"formatCurrency": func(val gopolls.CurrencyValue) string {
			return currencyHandler.Format(val)
		},
	}
	tFile, fileErr := pkger.Open("/cmd/poll/templates/base.html")
	if fileErr != nil {
		panic(fileErr)
	}
	content, readErr := ioutil.ReadAll(tFile)
	if readErr != nil {
		panic(readErr)
	}
	return template.Must(template.New("base.html").Parse(string(content))).Funcs(funcMap)

	// TODO there seems to be a bug in pkger with Dir, doesn't work this way, that's why we have the rather
	// "ugly" version above
	//return template.Must(vfstemplate.ParseFiles(pkger.Dir("/cmd/poll/templates"), nil,"base.html"))
}

func readTemplate(base *template.Template, name string) *template.Template {
	tFile, fileErr := pkger.Open("/cmd/poll/templates/" + name)
	if fileErr != nil {
		panic(fileErr)
	}
	content, readErr := ioutil.ReadAll(tFile)
	if readErr != nil {
		panic(readErr)
	}
	base = template.Must(base.Clone())
	template.Must(base.New(name).Parse(string(content)))
	return base

	// same as before, sadly there seems to be a bug in pkger
	// return template.Must(vfstemplate.ParseFiles(pkger.Dir("/cmd/poll/templates/"), template.Must(base.Clone()), names...))
}

func executeTemplate(t *template.Template, context *renderContext, buff *bytes.Buffer) handlerRes {
	templateErr := t.Execute(buff, context)
	if templateErr != nil {
		return newHandlerRes(http.StatusInternalServerError, templateErr)
	}

	return newHandlerRes(http.StatusOK, nil)
}

type mainHandler struct {
	template *template.Template
}

func newMainHandler(base *template.Template) *mainHandler {
	t := readTemplate(base, "index.html")
	return &mainHandler{t}
}

func (h *mainHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	renderContext := newRenderContext(context)
	return executeTemplate(h.template, renderContext, buff)
}

type votersHandler struct {
	template *template.Template
}

func newVotersHandler(base *template.Template) *votersHandler {
	t := readTemplate(base, "voters.html")
	return &votersHandler{t}
}

func (h *votersHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	renderContext := newRenderContext(context)

	render := func() handlerRes {
		return executeTemplate(h.template, renderContext, buff)
	}

	if r.Method == http.MethodGet {
		return render()
	}

	// already clear voters
	context.Voters = make([]*gopolls.Voter, 0, 0)
	context.VotersSourceFileName = ""
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return newHandlerRes(http.StatusInternalServerError, err)
	}

	// Actually check for ErrMissingFile here, but good enough for this
	file, handler, formErr := r.FormFile("voters-file")
	if formErr != nil {
		return newHandlerRes(http.StatusInternalServerError, formErr)
	}

	defer file.Close()

	// now try to parse from file
	voters, votersErr := gopolls.ParseVoters(file)
	if votersErr == nil {
		// if it is valid just redirect to voters page again
		context.Voters = voters
		context.VotersSourceFileName = handler.Filename
		log.Printf("Successfuly parsed %d voters from %s\n", len(voters), handler.Filename)
		res := newRedirectHandlerRes(http.StatusFound, "./")
		return res
	}

	// if an error occurred: if it is a syntax error render the error, otherwise return internal error
	if syntaxErr, ok := votersErr.(gopolls.PollingSyntaxError); ok {
		renderContext.AdditionalData["error"] = syntaxErr
		return render()
	}
	return newHandlerRes(http.StatusInternalServerError, votersErr)
}

type pollsHandler struct {
	template *template.Template
}

func newPollsHandler(base *template.Template) *pollsHandler {
	t := readTemplate(base, "polls.html")
	return &pollsHandler{t}
}

func (h *pollsHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	renderContext := newRenderContext(context)

	render := func() handlerRes {
		return executeTemplate(h.template, renderContext, buff)
	}

	if r.Method == http.MethodGet {
		return render()
	}

	// already clear polls
	context.PollCollection = gopolls.NewPollSkeletonCollection("dummy")
	context.CollectionSourceFileName = ""

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return newHandlerRes(http.StatusInternalServerError, err)
	}

	// Actually check for ErrMissingFile here, but good enough for this
	file, handler, formErr := r.FormFile("polls-file")
	if formErr != nil {
		return newHandlerRes(http.StatusInternalServerError, formErr)
	}

	defer file.Close()

	// now try to parse
	collection, collectionErr := gopolls.ParseCollectionSkeletons(file, currencyHandler)
	if collectionErr == nil {
		// just redirect to polls page again
		context.PollCollection = collection
		context.CollectionSourceFileName = handler.Filename
		log.Printf("Successfuly parsed %d polls from %s\n", collection.NumSkeletons(), handler.Filename)
		res := newRedirectHandlerRes(http.StatusFound, "./")
		return res
	}

	// if an error occurred: if it is a syntax error render the error, otherwise return internal error
	if syntaxErr, ok := collectionErr.(gopolls.PollingSyntaxError); ok {
		renderContext.AdditionalData["error"] = syntaxErr
		return render()
	}

	return newHandlerRes(http.StatusInternalServerError, collectionErr)
}

type exportCSVTemplateHandler struct{}

func newExportCSVTemplateHandler() exportCSVTemplateHandler {
	return exportCSVTemplateHandler{}
}

func (h exportCSVTemplateHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	csvWriter := gopolls.NewVotesCSVWriter(buff)
	// write empty template
	writeErr := csvWriter.GenerateEmptyTemplate(context.Voters, context.PollCollection.CollectSkeletons())
	if writeErr != nil {
		return newHandlerRes(http.StatusInternalServerError, writeErr)
	}
	res := newHandlerRes(http.StatusOK, nil)
	res.ContentType = "text/csv"
	return res
}

func main() {
	pkger.Include("/cmd/poll/templates")
	pkger.Include("/cmd/poll/static")

	base := baseTemplates()

	context := mainContext{}
	context.PollCollection = gopolls.NewPollSkeletonCollection("dummy")
	mainH := newMainHandler(base)
	votersH := newVotersHandler(base)
	pollsH := newPollsHandler(base)
	csvH := newExportCSVTemplateHandler()
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(pkger.Dir("/cmd/poll/static"))))
	http.HandleFunc("/voters/", toHandleFunc(votersH, &context))
	http.HandleFunc("/polls/", toHandleFunc(pollsH, &context))
	http.HandleFunc("/votes/csv/", toHandleFunc(csvH, &context))
	http.HandleFunc("/", toHandleFunc(mainH, &context))
	addr := "localhost:8080"
	log.Printf("Running server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
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
