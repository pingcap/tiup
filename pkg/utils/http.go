package utils

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
)

// PostFile upload file
func PostFile(reader io.Reader, url, fieldname, filename string) (*http.Response, error) {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// this step is very important
	fileWriter, err := bodyWriter.CreateFormFile(fieldname, filename)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(fileWriter, reader)
	if err != nil {
		return nil, err
	}

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post(url, contentType, bodyBuf)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
