package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/u2takey/ffmpeg-go"
)

func assertErr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func performRequest(t *testing.T, handle httprouter.Handle, route string, method string, code int, body io.Reader) string {
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(method, route, body)
	assertErr(t, err)

	handle(recorder, request, nil)
	if recorder.Code != code {
		t.Fatalf("bad status code: %d", recorder.Code)
	}
	return recorder.Body.String()
}

func checkErrMsg(t *testing.T, body string, expected string) {
	var response map[string]string
	err := json.Unmarshal([]byte(body), &response)
	assertErr(t, err)

	if response["message"] != expected {
		t.Fatalf("invalid error: %s", response["message"])
	}
}

func assertEqual(t *testing.T, v1 interface{}, v2 interface{}) {
	if !reflect.DeepEqual(v1, v2) {
		t.Fatalf("JSONs are different")
	}
}

// XXX: The test is quite dependent on exact JSON generation
// algorithm and should be improved.

func checkGeneratedJSONOutputStep(t *testing.T, jsonObj interface{}, depth int) (int, int) {
	numkeys := 0

	objType := reflect.TypeOf(jsonObj).Kind()

	lowerLevelCall := func(value interface{}) {
		lowerDepth, lowerNumkeys := checkGeneratedJSONOutputStep(t, value, depth)
		numkeys += lowerNumkeys
		if lowerDepth > depth {
			depth = lowerDepth
		}
	}

	switch objType {
	case reflect.Map:
		if len(jsonObj.(map[string]interface{})) != 0 {
			depth += 1
		}

		for _, value := range jsonObj.(map[string]interface{}) {
			numkeys++
			lowerLevelCall(value)
		}

	case reflect.Slice:
		for _, elem := range jsonObj.([]interface{}) {
			lowerLevelCall(elem)
		}

	case reflect.String:
		objectStr := jsonObj.(string)
		var jsonData map[string]interface{}
		err := json.Unmarshal([]byte(objectStr), &jsonData)
		if err == nil {
			lowerLevelCall(jsonData)
		}
	}
	return depth, numkeys
}

func checkGeneratedJSONOutput(t *testing.T, output string, levels int, numkeys int) {
	var jsonObj map[string]interface{}
	err := json.Unmarshal([]byte(output), &jsonObj)
	assertErr(t, err)

	depth, nkeys := checkGeneratedJSONOutputStep(t, jsonObj, 0)
	if depth != levels {
		t.Fatalf("generated json has %d levels instead of %d.", depth, levels)
	}

	if numkeys != nkeys {
		t.Fatalf("generated json has %d keys instead of %d.", nkeys, numkeys)
	}
	t.Logf("Generated JSON has %d levels and %d keys, as expected.", depth, nkeys)
}

func sortKeysSlice(json map[string]interface{}) {
	sort.Slice(json["keys"], func(i, j int) bool {
		return json["keys"].([]interface{})[i].(string) < json["keys"].([]interface{})[j].(string)
	})
}

func checkJSONOutput(t *testing.T, refFile string, output string) {
	data, err := os.ReadFile(refFile)
	assertErr(t, err)

	var expected map[string]interface{}
	var actual map[string]interface{}
	err = json.Unmarshal([]byte(output), &actual)
	assertErr(t, err)
	err = json.Unmarshal(data, &expected)
	assertErr(t, err)

	if _, ok := expected["keys"]; ok {
		if _, ok := actual["keys"]; ok {
			sortKeysSlice(expected)
			sortKeysSlice(actual)
			assertEqual(t, expected, actual)
		} else {
			t.Fatalf("Actual JSON doesn't have keys.")
		}
	} else {
		assertEqual(t, expected, actual)
	}
}

type testCaseWithRef struct {
	name   string
	in     string
	query  string
	ref    string
	status int
}

func runJSONReferenceSubtests(t *testing.T, testCases []testCaseWithRef, handle httprouter.Handle, method string) {
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			data, err := os.ReadFile(c.in)
			assertErr(t, err)

			dataReader := strings.NewReader(string(data))
			body := performRequest(t, handle, c.query, method, c.status, dataReader)
			checkJSONOutput(t, c.ref, body)
		})
	}
}

func TestGenerateJSONErrorChecks(t *testing.T) {
	testCases := []struct {
		name  string
		query string
		err   string
	}{
		{"no params", "/generate", "missing parameter `levels`"},
		{"no numkeys", "/generate?levels=3", "missing parameter `numkeys`"},
		{"no levels", "/generate?numkeys=10", "missing parameter `levels`"},
		{"invalid levels type", "/generate?levels=a&numkeys=3", "strconv.Atoi: parsing \"a\": invalid syntax"},
		{"invalid numkeys type", "/generate?levels=3&numkeys=b", "strconv.Atoi: parsing \"b\": invalid syntax"},
		{"levels greater than numkeys", "/generate?levels=3&numkeys=1", "`numkeys` must be greater than or equal to `levels`"},
		{"negative parameters", "/generate?levels=-2&numkeys=-1", "expected non-negative parameters"},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			body := performRequest(t, generateJSONHandle, c.query, "GET", http.StatusBadRequest, nil)
			checkErrMsg(t, body, c.err)
		})
	}
}

func TestGenerateJSONOutput(t *testing.T) {
	testCases := []struct {
		name    string
		query   string
		levels  int
		numkeys int
	}{
		{"standard", "generate?levels=4&numkeys=10", 4, 10},
		{"zero levels", "generate?levels=0&numkeys=10", 0, 0},
		{"zero parameters", "generate?levels=0&numkeys=0", 0, 0},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			body := performRequest(t, generateJSONHandle, c.query, "GET", http.StatusOK, nil)
			checkGeneratedJSONOutput(t, body, c.levels, c.numkeys)
		})
	}
}

func TestListJSONKeys(t *testing.T) {
	testCases := []testCaseWithRef{
		{"success", "test-assets/list-keys-input-success.json", "/keys", "test-assets/list-keys-output-success.json", http.StatusOK},
		{"faulty json", "test-assets/list-keys-input-fail.json", "/keys", "test-assets/list-keys-output-fail.json", http.StatusBadRequest},
	}
	runJSONReferenceSubtests(t, testCases, listJSONKeysHandle, "POST")
}

func TestFindValue(t *testing.T) {
	testCases := []testCaseWithRef{
		{"faulty JSON", "test-assets/find-value-input-fail.json", "/find", "test-assets/find-value-output-invalid-json.json", http.StatusBadRequest},
		{"missing parameter", "test-assets/find-value-input.json", "/find", "test-assets/find-value-output-missing.json", http.StatusBadRequest},
		{"empty", "test-assets/find-value-input.json", "/find?value=impossible", "test-assets/find-value-output-empty.json", http.StatusOK},
		{"number", "test-assets/find-value-input.json", "/find?value=200", "test-assets/find-value-output-number.json", http.StatusOK},
		{"bool", "test-assets/find-value-input.json", "/find?value=true", "test-assets/find-value-output-bool.json", http.StatusOK},
		{"string", "test-assets/find-value-input.json", "/find?value=cofax.tld", "test-assets/find-value-output-string.json", http.StatusOK},
		{"null", "test-assets/find-value-input.json", "/find?value=null", "test-assets/find-value-output-null.json", http.StatusOK},
	}
	runJSONReferenceSubtests(t, testCases, findValueHandle, "POST")
}

func TestConvertErrorCheck(t *testing.T) {
	testCases := []testCaseWithRef{
		{"faulty JSON", "test-assets/convert-input-fail.json", "/convert", "test-assets/convert-output-fail.json", http.StatusBadRequest},
		{"wrong format", "test-assets/convert-input-wrong-format.json", "/convert", "test-assets/convert-output-wrong-format.json", http.StatusBadRequest},
		{"not a string", "test-assets/convert-input-not-a-string.json", "/convert", "test-assets/convert-output-not-a-string.json", http.StatusBadRequest},
		{"decode failure", "test-assets/convert-input-decode.json", "/convert", "test-assets/convert-output-decode.json", http.StatusBadRequest},
		{"ffmpeg failure", "test-assets/convert-input-ffmpeg.json", "/convert", "test-assets/convert-output-ffmpeg.json", http.StatusBadRequest},
	}
	runJSONReferenceSubtests(t, testCases, convertHandle, "POST")
}

func TestConvert(t *testing.T) {
	testCases := []struct {
		name   string
		in     string
		ref    string
		status int
	}{
		{"wav", "test-assets/convert-input-wav.json", "audio/raw/song.wav", http.StatusOK},
		{"ogg", "test-assets/convert-input-ogg.json", "audio/raw/song.ogg", http.StatusOK},
		{"flac", "test-assets/convert-input-flac.json", "audio/raw/song.flac", http.StatusOK},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			err := ffmpeg_go.Input(c.ref).
				Output("out.tmp", ffmpeg_go.KwArgs{"c:v": "libmp3lame", "ac": 2, "b:a": "320k", "f": "mp3"}).
				OverWriteOutput().ErrorToStdOut().Run()
			assertErr(t, err)

			out, err := os.Open("out.tmp")
			assertErr(t, err)

			converted, err := encodeBase64FromFile(out)
			assertErr(t, err)

			err = os.Remove(out.Name())
			assertErr(t, err)

			expected := "{\"mp3\": \"" + converted + "\"}"

			data, err := os.ReadFile(c.in)
			assertErr(t, err)

			dataReader := strings.NewReader(string(data))
			actual := performRequest(t, convertHandle, "/convert", "POST", c.status, dataReader)
			if expected != actual {
				t.Fatalf("invalid conversion")
			}
		})
	}
}
