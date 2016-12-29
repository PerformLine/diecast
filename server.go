package diecast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghodss/yaml"
	"github.com/julienschmidt/httprouter"
	"github.com/op/go-logging"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var log = logging.MustGetLogger(`diecast`)

const DEFAULT_SERVE_ADDRESS = `127.0.0.1`
const DEFAULT_SERVE_PORT = 28419
const DEFAULT_ROUTE_PREFIX = `/`

var HeaderSeparator = []byte{'-', '-', '-'}

type TemplateHeader struct {
	Page     map[string]interface{} `json:"page,omitempty"`
	Bindings []Binding              `json:"bindings,omitempty"`
	Layout   string                 `json:"layout,omitempty"`
	Includes map[string]string      `json:"includes,omitempty"`
}

type Server struct {
	Address             string
	Port                int
	Bindings            []*Binding
	RootPath            string
	LayoutPath          string
	EnableLayouts       bool
	RoutePrefix         string
	TemplatePatterns    []string
	AdditionalFunctions template.FuncMap
	mounts              []Mount
	router              *httprouter.Router
	server              *negroni.Negroni
	fs                  http.FileSystem
	fsIsSet             bool
	fileServer          http.Handler
}

func NewServer(root string, patterns ...string) *Server {
	return &Server{
		Address:          DEFAULT_SERVE_ADDRESS,
		Port:             DEFAULT_SERVE_PORT,
		RoutePrefix:      DEFAULT_ROUTE_PREFIX,
		RootPath:         root,
		EnableLayouts:    true,
		Bindings:         make([]*Binding, 0),
		TemplatePatterns: patterns,
		mounts:           make([]Mount, 0),
	}
}

func (self *Server) SetMounts(mounts []Mount) {
	self.mounts = mounts
}

func (self *Server) Mounts() []Mount {
	return self.mounts
}

func (self *Server) SetFileSystem(fs http.FileSystem) {
	self.fs = fs
	self.fsIsSet = true
	self.fileServer = http.FileServer(self.fs)
}

func (self *Server) Initialize() error {
	// always make sure the root path is absolute
	if v, err := filepath.Abs(self.RootPath); err == nil {
		cwd, err := os.Getwd()

		if v == `./` && err == nil {
			self.RootPath = cwd
		} else {
			self.RootPath = v
		}
	} else {
		return err
	}

	if self.LayoutPath == `` {
		self.LayoutPath = path.Join(self.RootPath, `_layouts`)
	}

	self.RoutePrefix = strings.TrimSuffix(self.RoutePrefix, `/`)

	// if we haven't explicitly set a filesystem, create it
	if !self.fsIsSet {
		self.SetFileSystem(http.Dir(self.RootPath))
	}

	for _, binding := range self.Bindings {
		binding.server = self
	}

	if err := self.setupMounts(); err != nil {
		return err
	}

	if err := self.setupServer(); err != nil {
		return err
	}

	return nil
}

func (self *Server) Serve() {
	self.server.Run(fmt.Sprintf("%s:%d", self.Address, self.Port))
}

func (self *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	self.server.ServeHTTP(w, req)
}

func (self *Server) ShouldApplyTemplate(requestPath string) bool {
	baseName := filepath.Base(requestPath)

	for _, pattern := range self.TemplatePatterns {
		if match, err := filepath.Match(pattern, baseName); err == nil && match {
			return true
		}
	}

	return false
}

func (self *Server) ApplyTemplate(w http.ResponseWriter, requestPath string, reader io.Reader, header *TemplateHeader, data interface{}, layouts ...string) error {
	finalTemplate := bytes.NewBuffer(nil)
	hasLayout := false
	forceSkipLayout := false

	if header != nil {
		if header.Layout != `` {
			if header.Layout == `false` {
				forceSkipLayout = true
			} else {
				layouts = append([]string{header.Layout}, layouts...)
			}
		}
	}

	// only process layouts if we're supposed to
	if self.EnableLayouts && !forceSkipLayout {
		// files starting with "_" are partials and should not have layouts applied
		if !strings.HasPrefix(path.Base(requestPath), `_`) {
			// if no layouts were explicitly specified, and a layout named "default" exists, add it to the list
			if len(layouts) == 0 {
				if _, err := self.LoadLayout(`default`); err == nil {
					layouts = append(layouts, `default`)
				}
			}

			if len(layouts) > 0 {
				finalTemplate.WriteString("{{ define \"layout\" }}")

				for _, layoutName := range layouts {
					if layoutFile, err := self.LoadLayout(layoutName); err == nil {
						if _, err := io.Copy(finalTemplate, layoutFile); err != nil {
							return err
						}

						hasLayout = true
					} else {
						// we don't care if the default layout is missing
						if layoutName != `default` {
							return err
						}
					}
				}

				finalTemplate.WriteString("{{ end }}")
			}
		}
	}

	if hasLayout {
		finalTemplate.WriteString("{{ define \"content\" }}")
	}

	if _, err := io.Copy(finalTemplate, reader); err != nil {
		return err
	}

	if hasLayout {
		finalTemplate.WriteString("{{ end }}")
	}

	// create the template and make it aware of our custom functions
	tmpl := template.New(self.ToTemplateName(requestPath))
	tmpl.Funcs(GetStandardFunctions())

	if self.AdditionalFunctions != nil {
		tmpl.Funcs(self.AdditionalFunctions)
	}

	if tmpl, err := tmpl.Parse(finalTemplate.String()); err == nil {
		if hasLayout {
			return tmpl.ExecuteTemplate(w, `layout`, data)
		} else {
			return tmpl.Execute(w, data)
		}
	} else {
		return err
	}
}

func (self *Server) LoadLayout(name string) (io.Reader, error) {
	return os.Open(fmt.Sprintf("%s/%s.html", self.LayoutPath, name))
}

func (self *Server) ToTemplateName(requestPath string) string {
	requestPath = strings.Replace(requestPath, `/`, `-`, -1)

	return requestPath
}

func (self *Server) GetTemplateData(req *http.Request, header *TemplateHeader) (interface{}, error) {
	data := requestToEvalData(req)
	bindings := make(map[string]interface{})

	for _, binding := range self.Bindings {
		if v, err := binding.Evaluate(req); err == nil {
			bindings[binding.Name] = v
		} else {
			log.Warningf("Binding %q failed: %v", binding.Name, err)

			if !binding.Optional {
				return nil, err
			}
		}
	}

	if header != nil {
		for _, binding := range header.Bindings {
			binding.server = self

			if v, err := binding.Evaluate(req); err == nil {
				bindings[binding.Name] = v
			} else {
				log.Warningf("Binding %q failed: %v", binding.Name, err)

				if !binding.Optional {
					return nil, err
				}
			}
		}

		data[`page`] = header.Page
	}

	data[`bindings`] = bindings

	return data, nil
}

func (self *Server) handleFileRequest(w http.ResponseWriter, req *http.Request) {
	// normalize filename from request path
	requestPath := req.URL.Path

	// if we're looking at a directory, assume we want "index.html"
	if strings.HasSuffix(requestPath, `/`) {
		requestPath = path.Join(requestPath, `index.html`)
	}

	requestPaths := []string{
		requestPath,
	}

	// if we're requesting a path without a file extension, be a dear and try it with a .html
	// extension if the as-is path wasn't found
	if path.Ext(requestPath) == `` {
		requestPaths = append(requestPaths, fmt.Sprintf("%s.html", requestPath))
	}

	for _, rPath := range requestPaths {
		// remove the Route Prefix, as that's a structural part of the path but does not
		// represent where the files are (used for embedding diecast in other services
		// to avoid name collisions)
		//
		rPath = strings.TrimPrefix(rPath, self.RoutePrefix)

		log.Debugf("Requesting file %q", rPath)

		// find a mount that has this file
		for _, mount := range self.mounts {
			// attempt to open the file entry
			if file, err := mount.OpenFile(rPath); err == nil {
				// try to respond with the opened file
				if handled := self.respondToFile(rPath, file, w, req); handled {
					log.Debugf("  File %q was handled by mount %s", rPath, mount.MountPoint)
					return
				}
			} else {
				log.Debugf("  Skipping %q: failed to open file: %v", rPath, err)
			}
		}

		// if we got here, try to serve the file from the filesystem
		if file, err := os.Open(path.Join(self.RootPath, rPath)); err == nil {
			if handled := self.respondToFile(rPath, file, w, req); handled {
				log.Debugf("  File %q was handled by filesystem", rPath)
				return
			}
		}
	}

	// if we got *here*, then File Not Found
	http.Error(w, fmt.Sprintf("File %q was not found.", requestPath), http.StatusNotFound)
}

func (self *Server) respondToFile(requestPath string, file *os.File, w http.ResponseWriter, req *http.Request) bool {
	if stat, err := file.Stat(); err == nil {
		if !stat.IsDir() {
			log.Debugf("File %q -> %q", requestPath, file.Name())

			mimeType := `application/octet-stream`

			if v := mime.TypeByExtension(path.Ext(file.Name())); v != `` {
				mimeType = v
			}

			w.Header().Set(`Content-Type`, mimeType)

			// we got a real actual file here, figure out if we're templating it or not
			if self.ShouldApplyTemplate(requestPath) {
				log.Debugf("Rendering %q as template", requestPath)

				// tease the template header out of the file
				if header, templateData, err := self.SplitTemplateHeaderContent(file); err == nil {
					// load any included templates, add in their headers, and append them to the already-loaded template bytes
					if templateData, err := self.InjectIncludes(path.Dir(file.Name()), templateData, header); err == nil {
						// retrieve external data declared in the Bindings section
						if data, err := self.GetTemplateData(req, header); err == nil {
							// render the final template and write it out
							if err := self.ApplyTemplate(w, requestPath, bytes.NewBuffer(templateData), header, data); err != nil {
								http.Error(w, err.Error(), http.StatusInternalServerError)
							}
						} else {
							http.Error(w, err.Error(), http.StatusInternalServerError)
						}
					} else {
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			} else {
				io.Copy(w, file)
			}

			return true
		} else {
			// we know this is a directory, but the request didn't have a trailing slash
			// redirect
			if !strings.HasSuffix(req.URL.Path, `/`) {
				http.Redirect(w, req, fmt.Sprintf("%s/", req.URL.Path), http.StatusMovedPermanently)
				return true
			}
		}
	} else {
		log.Debugf("  Skipping %q: failed to stat file: %v", requestPath, err)
	}

	return false
}

func (self *Server) SplitTemplateHeaderContent(reader io.Reader) (*TemplateHeader, []byte, error) {
	if data, err := ioutil.ReadAll(reader); err == nil {
		if bytes.HasPrefix(data, HeaderSeparator) {
			parts := bytes.SplitN(data, HeaderSeparator, 3)

			if len(parts) == 3 {
				header := TemplateHeader{}

				if parts[1] != nil {
					if err := yaml.Unmarshal(parts[1], &header); err != nil {
						return nil, nil, err
					}
				}

				return &header, parts[2], nil
			}
		}

		return nil, data, nil
	} else {
		return nil, nil, err
	}
}

func (self *Server) InjectIncludes(cwd string, data []byte, baseHeader *TemplateHeader) ([]byte, error) {
	if baseHeader != nil {
		newData := data

		for name, includePath := range baseHeader.Includes {
			includePath = path.Clean(path.Join(cwd, includePath))

			if err := self.verifyRequestPathIsValid(includePath); err == nil {
				if file, err := os.Open(includePath); err == nil {

					if _, includeData, err := self.SplitTemplateHeaderContent(file); err == nil {
						// baseHeader = baseHeader.Merge(includeHeader)
						log.Debugf("Injecting included template %q from file %s", name, file.Name())

						define := fmt.Sprintf("{{ define %q }}", name)
						end := "{{ end }}"

						newData = append(newData, []byte(define)...)
						newData = append(newData, includeData...)
						newData = append(newData, []byte(end)...)
					} else {
						return data, err
					}
				}
			} else {
				return data, nil
			}
		}

		return newData, nil
	}

	return data, nil
}

func (self *Server) verifyRequestPathIsValid(validatePath string) error {
	if v, err := filepath.Abs(validatePath); err == nil {
		validatePath = v
	} else {
		return err
	}

	prefixInBounds := false
	validPrefixes := []string{
		self.RootPath,
	}

	for _, mount := range self.mounts {
		validPrefixes = append(validPrefixes, mount.Path)
	}

	for _, prefix := range validPrefixes {
		if v, err := filepath.Abs(prefix); err == nil {
			prefix = v
		} else {
			log.Warningf("Unable to get absolute path from %q: %v", prefix, err)
			continue
		}

		log.Debugf("Trying %q against: %s", validatePath, prefix)

		if strings.HasPrefix(validatePath, prefix) {
			prefixInBounds = true
			break
		}
	}

	if !prefixInBounds {
		return fmt.Errorf("Path %q is not a valid request path", validatePath)
	}

	return nil
}

func (self *Server) setupMounts() error {
	// initialize all mounts
	for _, mount := range self.mounts {
		if err := mount.Initialize(); err != nil {
			return err
		}
	}

	return nil
}

func (self *Server) setupServer() error {
	self.server = negroni.New()

	// setup panic recovery handler
	self.server.Use(negroni.NewRecovery())

	// setup internal/metadata routes
	mux := http.NewServeMux()

	mux.HandleFunc(fmt.Sprintf("%s/_diecast", self.RoutePrefix), func(w http.ResponseWriter, req *http.Request) {
		if data, err := json.Marshal(self); err == nil {
			w.Header().Set(`Content-Type`, `application/json`)

			if _, err := w.Write(data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc(fmt.Sprintf("%s/_bindings", self.RoutePrefix), func(w http.ResponseWriter, req *http.Request) {
		if data, err := json.Marshal(self.Bindings); err == nil {
			w.Header().Set(`Content-Type`, `application/json`)

			if _, err := w.Write(data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// all other routes proxy to this http.Handler
	mux.HandleFunc(fmt.Sprintf("%s/", self.RoutePrefix), self.handleFileRequest)

	self.server.UseHandler(mux)

	return nil
}

func requestToEvalData(req *http.Request) map[string]interface{} {
	rv := make(map[string]interface{})
	request := make(map[string]interface{})
	qs := make(map[string]interface{})
	hdr := make(map[string]interface{})

	for k, v := range req.URL.Query() {
		qs[k] = stringutil.Autotype(strings.Join(v, `, `))
	}

	for k, v := range req.Header {
		hdr[k] = stringutil.Autotype(strings.Join(v, `, `))
	}

	request[`method`] = req.Method
	request[`protocol`] = req.Proto
	request[`headers`] = hdr
	request[`length`] = req.ContentLength
	request[`encoding`] = req.TransferEncoding
	request[`remote_address`] = req.RemoteAddr
	request[`host`] = req.Host
	request[`url`] = map[string]interface{}{
		`unmodified`: req.RequestURI,
		`string`:     req.URL.String(),
		`scheme`:     req.URL.Scheme,
		`host`:       req.URL.Host,
		`path`:       req.URL.Path,
		`fragment`:   req.URL.Fragment,
		`query`:      qs,
	}

	rv[`request`] = request

	return rv
}
