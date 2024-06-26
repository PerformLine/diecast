//go:build amd64
// +build amd64

package diecast

import (
	"bytes"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/PerformLine/go-stockutil/typeutil"
	"github.com/stretchr/testify/require"
)

func TestSassRenderer(t *testing.T) {
	var assert = require.New(t)

	var server = NewServer(`./examples/hello-world`)
	var mounts = getTestMounts(assert)
	server.SetMounts(mounts)
	assert.NoError(server.Initialize())

	var renderer = new(SassRenderer)
	renderer.SetServer(server)

	var request = httptest.NewRequest(`GET`, `/css/for-sass.scss`, nil)
	var recorder = httptest.NewRecorder()

	var testsass = `$c1: red; $c2: blue; .parent { td { color: $c1; } tr { color: $c2 }}; @import '/css/for-sass';`

	assert.NoError(renderer.Render(recorder, request, RenderOptions{
		Input: ioutil.NopCloser(bytes.NewBufferString(testsass)),
	}))

	var res = recorder.Result()
	assert.NotNil(res)
	assert.Equal(`text/css; charset=utf-8`, res.Header.Get(`Content-Type`))
	assert.NotNil(res.Body)
	assert.Equal(".parent td {\n    color: red;\n}\n\n.parent tr {\n    color: blue;\n}\n\nh1 {\n    color: red;\n}\n", typeutil.String(res.Body))
	t.Logf("Test complete")
}
