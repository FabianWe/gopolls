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
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
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
	Matrix                   *gopolls.VotersMatrix
	MatrixSourceFileName     string
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

	// if an error occurred: if it is a syntax error or duplicate error render the error, otherwise return internal
	// error
	switch votersErr.(type) {
	case gopolls.PollingSyntaxError, gopolls.DuplicateError:
		renderContext.AdditionalData["error"] = votersErr
		return render()
	default:
		return newHandlerRes(http.StatusInternalServerError, votersErr)
	}

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

	// if an error occurred: if it is a syntax error render the error, otherwise return internal error
	if syntaxErr, ok := collectionErr.(gopolls.PollingSyntaxError); ok {
		renderContext.AdditionalData["error"] = syntaxErr
		return render()
	}

	return newHandlerRes(http.StatusInternalServerError, collectionErr)
}

type evaluationHandler struct {
	template *template.Template
}

func newEvaluationHandler(base *template.Template) *evaluationHandler {
	t := readTemplate(base, "evaluate.gohtml")
	return &evaluationHandler{t}
}

func (h *evaluationHandler) handleWithMatrix(fileName string, matrix *gopolls.VotersMatrix, ctx *renderContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	voters, pollSkells, matrixErr := matrix.PrepareAndVerifyVotesMatrix()
	if matrixErr != nil {
		log.Println("Error verifying matrix:", matrixErr)
	}
	// translate skeletons to polls
	convertFunction := gopolls.NewDefaultSkeletonConverter(true)
	polls, convertErr := gopolls.ConvertSkeletonsToPolls(pollSkells,
		convertFunction)
	if convertErr != nil {
		// something is wrong
		return newHandlerRes(http.StatusInternalServerError, convertErr)
	}

	fmt.Println("Jup", voters, polls)
	return newHandlerRes(http.StatusOK, nil)
}

func (h *evaluationHandler) handleMatrixErr(fileName string, err error, ctx *renderContext, buff *bytes.Buffer, r *http.Request) handlerRes {
	// internal errors (we need a nicer way of doing this) are reported as error, otherwise an internal server
	// error is returned
	switch err.(type) {
	case gopolls.PollingSyntaxError, gopolls.DuplicateError, gopolls.SkelTypeConversionError:
		ctx.AdditionalData["error"] = err
		return executeTemplate(h.template, ctx, buff)
	}
	return newHandlerRes(http.StatusInternalServerError, err)
}

func (h *evaluationHandler) Handle(context *mainContext, buff *bytes.Buffer, r *http.Request) handlerRes {

	renderContext := newRenderContext(context)

	if r.Method == http.MethodGet {
		return executeTemplate(h.template, renderContext, buff)
	}

	// test that both voters and polls are not empty
	if len(context.Voters) == 0 || context.PollCollection.NumSkeletons() == 0 {
		return h.handleMatrixErr("", nil,
			renderContext, buff, r)
	}

	// already clear matrix
	context.Matrix = nil
	context.MatrixSourceFileName = ""
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return newHandlerRes(http.StatusInternalServerError, err)
	}

	file, handler, formErr := r.FormFile("matrix-file")
	if formErr != nil {
		return newHandlerRes(http.StatusInternalServerError, formErr)
	}

	defer file.Close()

	// now try to parse the matrix and validate it
	matrix, matrixErr := gopolls.NewVotersMatrixFromCSV(file, context.Voters, context.PollCollection)
	if matrixErr != nil {
		return h.handleMatrixErr(handler.Filename, matrixErr, renderContext, buff, r)
	}

	return h.handleWithMatrix(handler.Filename, matrix, renderContext, buff, r)

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

func parseArgs() {
	var rootString string
	flag.StringVar(&rootString, "assets", "", "Directory in which the assets (templates and static) are, defaults to dir of executable")
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
