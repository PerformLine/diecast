package diecast

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/sliceutil"
)

var DefaultProxyMountTimeout = time.Duration(10) * time.Second

type ProxyMount struct {
	MountPoint          string            `json:"-"`
	URL                 string            `json:"-"`
	Method              string            `json:"method,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	Timeout             time.Duration     `json:"timeout,omitempty"`
	PassthroughRequests bool              `json:"passthrough_requests"`
	PassthroughErrors   bool              `json:"passthrough_errors"`
	Client              *http.Client
}

func (self *ProxyMount) GetMountPoint() string {
	return self.MountPoint
}

func (self *ProxyMount) WillRespondTo(name string, req *http.Request, requestBody io.Reader) bool {
	return strings.HasPrefix(name, self.GetMountPoint())
}

func (self *ProxyMount) OpenWithType(name string, req *http.Request, requestBody io.Reader) (*MountResponse, error) {
	var proxyURI string

	if self.Client == nil {
		if self.Timeout == 0 {
			self.Timeout = DefaultProxyMountTimeout
		}

		self.Client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
			Timeout: self.Timeout,
		}
	}

	if self.Method == `` {
		self.Method = `get`
	}

	if req != nil && self.PassthroughRequests {
		if newURL, err := url.Parse(self.URL); err == nil {
			req.URL.Scheme = newURL.Scheme
			req.URL.Host = newURL.Host

			if newURL.User != nil {
				req.URL.User = newURL.User
			}

			if newURL.Fragment != `` {
				req.URL.Fragment = newURL.Fragment
			}

			// merge incoming query strings with proxy query strings
			qs := req.URL.Query()

			for newQs, newVs := range newURL.Query() {
				for _, v := range newVs {
					qs.Add(newQs, v)
				}
			}

			req.URL.RawQuery = qs.Encode()

			proxyURI = req.URL.String()
		} else {
			return nil, fmt.Errorf("Failed to parse proxy URL: %v", err)
		}
	} else {
		proxyURI = strings.Join([]string{
			strings.TrimSuffix(self.URL, `/`),
			strings.TrimPrefix(name, `/`),
		}, `/`)
	}

	method := strings.ToUpper(self.Method)

	if req != nil && self.PassthroughRequests {
		method = req.Method
	}

	if newReq, err := http.NewRequest(method, proxyURI, nil); err == nil {
		if req != nil && self.PassthroughRequests {
			for name, values := range req.Header {
				for _, value := range values {
					newReq.Header.Set(name, value)
				}
			}
		}

		for name, value := range self.Headers {
			newReq.Header.Set(name, value)
		}

		if requestBody != nil && self.PassthroughRequests {
			newReq.Body = ioutil.NopCloser(requestBody)
		}

		log.Infof("  proxying '%v %v' to '%v %v'", req.Method, req.URL, newReq.Method, proxyURI)

		log.Debugf("  %v: %v %v", self, newReq.Method, newReq.URL)

		for k, v := range newReq.Header {
			log.Debugf("  %v: [H] %v=%v", self, k, strings.Join(v, ` `))
		}

		if response, err := self.Client.Do(newReq); err == nil {
			log.Debugf("  %v: [R] %v", self, response.Status)

			for k, v := range response.Header {
				log.Debugf("  %v: [R]   %v: %v", self, k, strings.Join(v, ` `))
			}

			log.Infof(
				"%v: %v %v responded with: %v (Content-Length: %v)",
				self,
				newReq.Method,
				proxyURI,
				response.Status,
				response.ContentLength,
			)

			if response.StatusCode < 400 || self.PassthroughErrors {
				if data, err := ioutil.ReadAll(response.Body); err == nil {
					payload := bytes.NewReader(data)
					mountResponse := NewMountResponse(name, payload.Size(), payload)
					mountResponse.StatusCode = response.StatusCode
					mountResponse.ContentType = response.Header.Get(`Content-Type`)

					for k, v := range response.Header {
						mountResponse.Metadata[k] = strings.Join(v, `,`)
					}

					return mountResponse, nil
				} else {
					return nil, err
				}
			} else {
				log.Debugf("  %v: %s %s: %s", self, method, proxyURI, response.Status)
				return nil, MountHaltErr
			}
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (self *ProxyMount) String() string {
	return fmt.Sprintf(
		"%v -> %v %v (passthrough requests=%v errors=%v)",
		self.MountPoint,
		strings.ToUpper(sliceutil.OrString(self.Method, `get`)),
		self.URL,
		self.PassthroughRequests,
		self.PassthroughErrors,
	)
}

func (self *ProxyMount) Open(name string) (http.File, error) {
	return openAsHttpFile(self, name)
}
