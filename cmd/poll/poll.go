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
	"errors"
	"flag"
	"fmt"
	"github.com/FabianWe/gopolls"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"
)

const version = "0.0.1"

var currencyHandler = gopolls.SimpleEuroHandler{}

// used to store the "root" path for static files and templates, avoid passing it around as argument
// should be fine enough in this main file
var templateRoot string
var staticRoot string

type mainContext struct {
	Voters         []*gopolls.Voter
	PollCollection *gopolls.PollSkeletonCollection
	// in case voters were loaded from a file this value is set to the name
	VotersSourceFileName string
	// in case collection was loaded from a file this value is set to this path
	CollectionSourceFileName string

	// if you're reading this: don't do this in any live code, it's only here for this app, you would never do that
	// because this is a small demonstration that should be used nowhere I think it will be fine
	mutex sync.Mutex
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
	FileName    string
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
		start := time.Now()
		// as mentioned before: never do things this way, just for the sake of this sample demo app
		context.mutex.Lock()
		defer context.mutex.Unlock()
		handlerRes := h.Handle(context, &buff, r)
		delta := time.Since(start)
		log.Println("Handler done after", delta)
		if handlerRes.ContentType != "" {
			w.Header().Set("Content-Type", handlerRes.ContentType)
			if handlerRes.FileName != "" {
				w.Header().Set("Content-Disposition",
					fmt.Sprintf("attachment; filename=%s", handlerRes.FileName))
			}

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

	basePath := filepath.Join(templateRoot, "base.gohtml")
	base := template.Must(template.ParseFiles(basePath))
	return base.Funcs(funcMap)
}

func readTemplate(base *template.Template, name string) *template.Template {
	templatePath := filepath.Join(templateRoot, name)
	return template.Must(template.Must(base.Clone()).ParseFiles(templatePath))
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
	t := readTemplate(base, "index.gohtml")
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
	t := readTemplate(base, "voters.gohtml")
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
		// check for duplicate names, if there are any set error to a duplicate error
		if name, hasDuplicates := gopolls.HasDuplicateVoters(voters); hasDuplicates {
			votersErr = gopolls.NewDuplicateError(fmt.Sprintf("duplicate voter name %s", name))
		}
	}

	if votersErr == nil {
		// if it is valid just redirect to voters page again
		context.Voters = voters
		context.VotersSourceFileName = handler.Filename
		log.Printf("Successfuly parsed %d voters from %s\n", len(voters), handler.Filename)
		res := newRedirectHandlerRes(http.StatusFound, "./")
		return res
	}

	// if an error occurred: if it is an internal gopolls error render it
	if errors.Is(votersErr, gopolls.ErrPoll) {
		renderContext.AdditionalData["error"] = votersErr
		return render()
	}

	return newHandlerRes(http.StatusInternalServerError, votersErr)
}

type pollsHandler struct {
	template *template.Template
}

func newPollsHandler(base *template.Template) *pollsHandler {
	t := readTemplate(base, "polls.gohtml")
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
		// now check for duplicate names in the polls, if there are any set error to a duplicate error
		if name, hasDuplicates := collection.HasDuplicateSkeleton(); hasDuplicates {
			collectionErr = gopolls.NewDuplicateError(fmt.Sprintf("duplicate poll name %s", name))
		}
	}

	if collectionErr == nil {
		// just redirect to polls page again
		context.PollCollection = collection
		context.CollectionSourceFileName = handler.Filename
		log.Printf("Successfuly parsed %d polls from %s\n", collection.NumSkeletons(), handler.Filename)
		res := newRedirectHandlerRes(http.StatusFound, "./")
		return res
	}

	// if an error occurred: if it is a gopoll internal error display it
	if errors.Is(collectionErr, gopolls.ErrPoll) {
		renderContext.AdditionalData["error"] = collectionErr
		return render()
	}

	return newHandlerRes(http.StatusInternalServerError, collectionErr)
}

type evaluationHandler struct {
	template                  *template.Template
	evaluationResultsTemplate *template.Template
}

func newEvaluationHandler(base *template.Template) *evaluationHandler {
	standardTemplate := readTemplate(base, "evaluate.gohtml")
	evaluationResultsTemplate := readTemplate(base, "evaluation_results.gohtml")
	return &evaluationHandler{
		template:                  standardTemplate,
		evaluationResultsTemplate: evaluationResultsTemplate,
	}
}

func (h *evaluationHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {

	renderContext := newRenderContext(context)

	render := func(err error) handlerRes {
		if err == nil {
			return executeTemplate(h.template, renderContext, buff)
		}
		if errors.Is(err, gopolls.ErrPoll) {
			renderContext.AdditionalData["error"] = err
			return executeTemplate(h.template, renderContext, buff)
		}
		return newHandlerRes(http.StatusInternalServerError, err)
	}

	if r.Method == http.MethodGet {
		return render(nil)
	}

	// try to read the matrix
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return newHandlerRes(http.StatusInternalServerError, err)
	}

	file, handler, formErr := r.FormFile("matrix-file")
	if formErr != nil {
		return newHandlerRes(http.StatusInternalServerError, formErr)
	}

	defer file.Close()

	// try to parse the matrix
	csvReader := gopolls.NewVotesCSVReader(file)
	csvReader.Sep = ';'
	matrix, matrixErr := gopolls.NewVotersMatrixFromCSV(csvReader, context.Voters, context.PollCollection)
	if matrixErr != nil {
		return render(matrixErr)
	}

	// matrix has been parsed, so now try to parse the votes from it
	polls, pollsErr := gopolls.ConvertSkeletonsToPolls(matrix.Polls,
		gopolls.DefaultSkeletonConverter)
	if pollsErr != nil {
		return render(pollsErr)
	}

	// next try to parse the results, first generate the parsers
	parsers, parsersErr := gopolls.GenerateDefaultParsers(polls, nil, nil, nil)
	if parsersErr != nil {
		return render(parsersErr)
	}

	fmt.Println(parsers, handler.Filename)
	return newHandlerRes(http.StatusOK, nil)

}

type exportCSVTemplateHandler struct{}

func newExportCSVTemplateHandler() exportCSVTemplateHandler {
	return exportCSVTemplateHandler{}
}

func (h exportCSVTemplateHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	csvWriter := gopolls.NewVotesCSVWriter(buff)
	csvWriter.Sep = ';'
	// write empty template
	writeErr := csvWriter.GenerateEmptyTemplate(context.Voters, context.PollCollection.CollectSkeletons())
	if writeErr != nil {
		return newHandlerRes(http.StatusInternalServerError, writeErr)
	}
	res := newHandlerRes(http.StatusOK, nil)
	res.ContentType = "text/csv"
	res.FileName = "votes.csv"
	return res
}

func main() {
	//pkger.Include("/cmd/poll/templates")
	//pkger.Include("/cmd/poll/static")
	parseArgs()

	base := baseTemplates()

	context := mainContext{}
	context.PollCollection = gopolls.NewPollSkeletonCollection("dummy")
	mainH := newMainHandler(base)
	votersH := newVotersHandler(base)
	pollsH := newPollsHandler(base)
	csvH := newExportCSVTemplateHandler()
	evaluateH := newEvaluationHandler(base)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticRoot))))
	http.HandleFunc("/voters/", toHandleFunc(votersH, &context))
	http.HandleFunc("/polls/", toHandleFunc(pollsH, &context))
	http.HandleFunc("/votes/csv/", toHandleFunc(csvH, &context))
	http.HandleFunc("/evaluate/", toHandleFunc(evaluateH, &context))
	http.HandleFunc("/", toHandleFunc(mainH, &context))
	addr := "localhost:8080"
	log.Printf("Running server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func doesDirExist(path string) bool {
	stat, err := os.Stat(path)

	if err != nil {
		if os.IsExist(err) {
			return false
		}
		log.Fatalf("error accessing assets directory %s: %v", path, err)
	}
	if !stat.IsDir() {
		log.Fatalf("%s is a file, not a directory", path)
	}
	return true
}

const copyrightStr = `Copyright 2020 Fabian Wenzelmann <fabianwen@posteo.eu>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.`

const projectURL = "https://github.com/FabianWe/gopolls"

func printUsage() {
	prog := os.Args[0]
	flag.CommandLine.SetOutput(os.Stdout)
	// write usage
	fmt.Printf("Use \"%s help\" to display this message\n", prog)
	fmt.Printf("Use \"%s about\" to print copyright and meta information\n\n", prog)
	fmt.Printf("Options for %s:\n\n", prog)
	flag.PrintDefaults()
}

func printAbout() {
	fmt.Printf("This is gopolls version %s (Go version %s)\n\n", version, runtime.Version())
	fmt.Println(copyrightStr)
	fmt.Printf("\nAdditional information such as third-party licesnses and usage\ninformation can be found on the project homepage at\n\t%s\n", projectURL)
}

func parseArgs() {
	var rootString string
	flag.StringVar(&rootString, "assets", "", "Directory in which the assets (templates and static) are, defaults to dir of executable")
	// test if help was given
	if len(os.Args) > 1 && os.Args[1] == "help" {
		printUsage()
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "about" {
		printAbout()
		os.Exit(0)
	}
	flag.Parse()
	if rootString == "" {
		// try to get executable directory
		execPath, err := os.Executable()
		if err == nil {
			rootString = filepath.Dir(execPath)
		} else {
			rootString = "./"
			log.Println("Can't determine executable directory, assuming assets are in ./")
		}
	}
	// check if directories exist
	templateDir := filepath.Join(rootString, "templates")
	staticDir := filepath.Join(rootString, "static")

	if !doesDirExist(templateDir) {
		log.Fatalf("template directory does not exist, assumed it to be at %s", templateDir)
	}

	if !doesDirExist(staticDir) {
		log.Fatalf("static directory does not exist, assumed it to be at %s", templateDir)
	}

	templateRoot = templateDir
	staticRoot = staticDir
}
