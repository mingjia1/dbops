package controllers

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicensePayloadReadsMultipartLicenseFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("license", "license.json")
	require.NoError(t, err)
	_, err = file.Write([]byte(`{"license_key":"test"}`))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest("POST", "/license/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	payload, err := licensePayload(ctx)

	require.NoError(t, err)
	assert.JSONEq(t, `{"license_key":"test"}`, string(payload))
}

func TestLicensePayloadReadsRawJSON(t *testing.T) {
	request := httptest.NewRequest("POST", "/license/upload", bytes.NewBufferString(`{"license_key":"test"}`))
	request.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request

	payload, err := licensePayload(ctx)

	require.NoError(t, err)
	assert.JSONEq(t, `{"license_key":"test"}`, string(payload))
}
