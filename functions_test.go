package diecast

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCollectionFunctions(t *testing.T) {
	var assert = require.New(t)
	var fns = GetStandardFunctions(nil)

	var page = fns[`page`].(func(interface{}, interface{}) int)

	assert.Zero(page(0, 0))
	assert.Zero(page(0, -25))
	assert.Zero(page(0, 25))
	assert.Zero(page(-1, 0))
	assert.Zero(page(0, -1))
	assert.Zero(page(1, 1))
	assert.Equal(0, page(1, 25))
	assert.Equal(25, page(2, 25))
	assert.Equal(50, page(3, 25))
	assert.Equal(75, page(4, 25))
	assert.Equal(100, page(5, 25))

	var isLastElement = fns[`isLastElement`].(func(index interface{}, array interface{}) bool)
	var arr = []string{`a`, `b`, `c`}

	assert.False(isLastElement(-1, arr), arr)
	assert.False(isLastElement(0, arr), arr)
	assert.False(isLastElement(1, arr), arr)
	assert.True(isLastElement(2, arr), arr)
	assert.False(isLastElement(3, arr), arr)

	var longestString = fns[`longestString`].(func(interface{}) string)
	assert.Equal(`three`, longestString([]string{`one`, `two`, `three`, `four`, `five`}))
	assert.Equal(`four`, longestString([]string{`one`, `two`, `four`, `five`}))
	assert.Equal(`five`, longestString([]string{`one`, `two`, `five`}))
	assert.Equal(`one`, longestString([]string{`one`, `two`}))
}

func TestCollectionFunctionsCodecs(t *testing.T) {
	var assert = require.New(t)
	var fns = GetStandardFunctions(nil)

	var chr2str = fns[`chr2str`].(func(codepoints interface{}) string)

	assert.Equal(`HELLO`, chr2str([]uint8{72, 69, 76, 76, 79}))
	assert.Equal(`THERE`, chr2str([]uint8{84, 72, 69, 82, 69}))
}

func TestTimeFunctions(t *testing.T) {
	var assert = require.New(t)
	var fns = GetStandardFunctions(nil)

	var isOlderThan = fns[`isOlderThan`].(func(t interface{}, d interface{}) (bool, error))
	var isNewerThan = fns[`isNewerThan`].(func(t interface{}, d interface{}) (bool, error))

	b, err := isOlderThan(time.Now().Add(-1*time.Hour), "30m")
	assert.NoError(err)
	assert.True(b)

	b, err = isOlderThan(time.Now().Add(-1*time.Hour), "2h")
	assert.NoError(err)
	assert.False(b)

	b, err = isNewerThan(time.Now().Add(-1*time.Hour), "30m")
	assert.NoError(err)
	assert.False(b)

	b, err = isNewerThan(time.Now().Add(-1*time.Hour), "2h")
	assert.NoError(err)
	assert.True(b)
}

func TestMiscFunctions(t *testing.T) {
	var assert = require.New(t)
	var fns = GetStandardFunctions(nil)

	var fn_switch = fns[`switch`].(func(input interface{}, fallback interface{}, pairs ...interface{}) interface{})

	assert.Equal(`1`, fn_switch(`a`, `fallback`, `a`, `1`, `b`, `2`))
	assert.Equal(`2`, fn_switch(`b`, `fallback`, `a`, `1`, `b`, `2`))
	assert.Equal(`fallback`, fn_switch(`c`, `fallback`, `a`, `1`, `b`, `2`))

	var fn_random = fns[`random`].(func(bounds ...interface{}) int64)

	for i := 0; i < 100000; i++ {
		var v = fn_random()
		assert.True(v >= 0 && v < math.MaxInt64)
	}

	for i := 0; i < 100000; i++ {
		var v = fn_random(42)
		assert.True(v >= 42 && v < math.MaxInt64)
	}

	for i := 0; i < 100000; i++ {
		var v = fn_random(42, 96)
		assert.True(v >= 42 && v < 96)
	}

	for i := 0; i < 100000; i++ {
		var v = fn_random(-100, 101)
		assert.True(v >= -100 && v < 101)
	}
}

// TODO: Wanna make this idea work...
//
// func TestDocExamples(t *testing.T) {
// 	assert := optassert.New(t)
// 	docGroups, funcs := GetFunctions(nil)

// 	for _, group := range docGroups {
// 		for _, fnDoc := range group.Functions {
// 			for _, example := range fnDoc.Examples {
// 				name := fmt.Sprintf("test:fn(%s):%s", group.Name, fnDoc.Name)

// 				tpl := NewTemplate(name, TextEngine)
// 				tpl.Funcs(funcs)

// 				assert.NoError(tpl.ParseString(`{{ `+example.Code+`}}`), name)
// 			}
// 		}
// 	}
// }
