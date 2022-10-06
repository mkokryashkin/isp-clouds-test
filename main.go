package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/u2takey/ffmpeg-go"
)

func gracefulFail(w http.ResponseWriter, code int, errmsg string) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "{\"message\": %q}", errmsg)
	log.Output(2, errmsg)
}

func decodeJSON(r *http.Request) (map[string]interface{}, error) {
	var jsonData map[string]interface{}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return jsonData, err
	}

	err = json.Unmarshal(body, &jsonData)
	return jsonData, err
}

func encodeJSONKeysArray(keys []string) (string, error) {
	if len(keys) == 0 {
		return "{\"keys\": []}", nil
	}

	responseArray, err := json.Marshal(keys)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("{\"keys\": %s}", string(responseArray)), nil
}

func generateJSON(levels int, numkeys int) string {
	if levels == 0 || numkeys == 0 {
		return "{}"
	}

	result := "{"
	for i := 0; i < levels-1; i++ {
		curLevel := fmt.Sprintf("\"%d\": {", rand.Int())
		result += curLevel
	}

	lastLevel := fmt.Sprintf("\"%d\": \"%d\"", rand.Int(), rand.Int())
	result += lastLevel + strings.Repeat("}", levels-1)

	for i := 0; i < numkeys-levels; i++ {
		result += fmt.Sprintf(", \"%d\": \"%d\"", rand.Int(), rand.Int())
	}
	result += "}"

	return result
}

func searchValueInJsonObject(object interface{}, needle string, parentKey string) []string {
	FLOAT_PRECISION := 64
	EPS := 0.001

	if object == nil {
		var res []string
		if needle == "null" {
			res = append(res, parentKey)
		}
		return res
	}

	var keys []string
	objectType := reflect.TypeOf(object).Kind()

	switch objectType {
	case reflect.Map:
		for k, v := range object.(map[string]interface{}) {
			keys = append(keys, searchValueInJsonObject(v, needle, k)...)
		}

	case reflect.String:
		objectStr := object.(string)

		if objectStr == needle && parentKey != "" {
			keys = append(keys, parentKey)
		} else {
			var jsonData map[string]interface{}
			err := json.Unmarshal([]byte(objectStr), &jsonData)
			if err == nil {
				keys = append(keys, searchValueInJsonObject(jsonData, needle, "")...)
			}
		}

	case reflect.Slice:
		for _, elem := range object.([]interface{}) {
			keys = append(keys, searchValueInJsonObject(elem, needle, "")...)
		}

	case reflect.Bool:
		if strconv.FormatBool(object.(bool)) == needle {
			keys = append(keys, parentKey)
		}

	case reflect.Float64:
		res, err := strconv.ParseFloat(needle, FLOAT_PRECISION)
		if err == nil && math.Abs(res-object.(float64)) < EPS {
			keys = append(keys, parentKey)
		}

	default:
		log.Fatal("Incorrect unmarshaling")
	}

	return keys
}

func listJSONKeysHandle(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	jsonData, err := decodeJSON(r)
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	keys := make([]string, 0, len(jsonData))
	for key, _ := range jsonData {
		keys = append(keys, key)
	}

	if jsonResp, err := encodeJSONKeysArray(keys); err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	} else {
		fmt.Fprint(w, jsonResp)
	}
}

func generateJSONHandle(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	levelsArr := query["levels"]
	if len(levelsArr) == 0 {
		gracefulFail(w, http.StatusBadRequest, "missing parameter `levels`")
		return
	}

	numkeysArr := query["numkeys"]
	if len(numkeysArr) == 0 {
		gracefulFail(w, http.StatusBadRequest, "missing parameter `numkeys`")
		return
	}

	levels, err := strconv.Atoi(levelsArr[0])
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	numkeys, err := strconv.Atoi(numkeysArr[0])
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	if levels > numkeys {
		gracefulFail(w, http.StatusBadRequest, "`numkeys` must be greater than or equal to `levels`")
		return
	}

	if levels < 0 || numkeys < 0 {
		gracefulFail(w, http.StatusBadRequest, "expected non-negative parameters")
		return
	}
	res := generateJSON(levels, numkeys)

	fmt.Fprintf(w, "%s", res)
}

func findValueHandle(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	jsonData, err := decodeJSON(r)
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	valueArr := query["value"]
	if len(valueArr) == 0 {
		gracefulFail(w, http.StatusBadRequest, "missing parameter `value`")
		return
	}
	value := valueArr[0]
	keys := searchValueInJsonObject(jsonData, value, "")

	if jsonResp, err := encodeJSONKeysArray(keys); err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	} else {
		fmt.Fprint(w, jsonResp)
	}
}

func decodeBase64ToFile(encoded string, file *os.File) error {
	defer file.Close()
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}

	if _, err := file.Write(decoded); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return nil
}

func encodeBase64FromFile(file *os.File) (string, error) {
	defer file.Close()
	reader := bufio.NewReader(file)

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	return encoded, nil
}

func convertHandle(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	jsonData, err := decodeJSON(r)
	if err != nil {
		gracefulFail(w, http.StatusBadRequest, err.Error())
		return
	}

	for _, format := range []string{"flac", "ogg", "wav"} {
		encoded, ok := jsonData[format]
		if !ok {
			continue
		}

		if reflect.TypeOf(encoded).Kind() != reflect.String {
			gracefulFail(w, http.StatusBadRequest, "not a base64")
			return
		}

		encodedStr := encoded.(string)

		in, err := os.CreateTemp("", "input")
		if err != nil {
			log.Fatal(err)
		}

		out, err := os.CreateTemp("", "output")
		if err != nil {
			log.Fatal(err)
		}

		if err := decodeBase64ToFile(encodedStr, in); err != nil {
			gracefulFail(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := ffmpeg_go.Input(in.Name()).
			Output(out.Name(), ffmpeg_go.KwArgs{"c:v": "libmp3lame", "ac": 2, "b:a": "320k", "f": "mp3"}).
			OverWriteOutput().ErrorToStdOut().Run(); err != nil {
			gracefulFail(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := os.Remove(in.Name()); err != nil {
			log.Fatal(err)
		}

		converted, err := encodeBase64FromFile(out)
		if err != nil {
			log.Fatal(err)
		}

		if err := os.Remove(out.Name()); err != nil {
			log.Fatal(err)
		}

		fmt.Fprintf(w, "{\"mp3\": \"%s\"}", converted)
		return
	}
	gracefulFail(w, http.StatusBadRequest, "no audio data provided")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	rand.Seed(time.Now().UnixNano())

	router := httprouter.New()
	router.POST("/keys", listJSONKeysHandle)
	router.GET("/generate", generateJSONHandle)
	router.POST("/find", findValueHandle)
	router.POST("/convert", convertHandle)

	log.Fatal(http.ListenAndServe(":8080", router))
}
