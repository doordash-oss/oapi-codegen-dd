// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ json.Marshaler = (*File)(nil)
var _ json.Unmarshaler = (*File)(nil)

func TestFileJSON(t *testing.T) {
	type Object struct {
		BinaryField File `json:"binary_field"`
	}

	// Check whether we encode JSON properly.
	var o Object
	o.BinaryField.InitFromBytes([]byte("hello"), "")
	buf, err := json.Marshal(o)
	require.NoError(t, err)
	t.Log(string(buf))

	// Decode the same object back into File, ensure result is correct.
	var o2 Object
	err = json.Unmarshal(buf, &o2)
	require.NoError(t, err)
	o2Bytes, err := o2.BinaryField.Bytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), o2Bytes)

	// Ensure it also works via pointer.
	type Object2 struct {
		BinaryFieldPtr *File `json:"binary_field"`
	}

	var o3 Object2
	var f File
	f.InitFromBytes([]byte("hello"), "")
	o3.BinaryFieldPtr = &f
	buf, err = json.Marshal(o)
	require.NoError(t, err)
	t.Log(string(buf))

	var o4 Object2
	err = json.Unmarshal(buf, &o4)
	require.NoError(t, err)
	o4Bytes, err := o4.BinaryFieldPtr.Bytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), o4Bytes)

}

func TestFileContentType(t *testing.T) {
	tests := []struct {
		name   string
		header textproto.MIMEHeader
		want   string
	}{
		{
			name:   "declared type",
			header: textproto.MIMEHeader{"Content-Type": {"image/jpeg"}},
			want:   "image/jpeg",
		},
		{
			name:   "type with parameters is passed through whole",
			header: textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}},
			want:   "text/plain; charset=utf-8",
		},
		{
			name:   "header spelled in another case",
			header: textproto.MIMEHeader{"content-type": {"image/png"}},
			want:   "image/png",
		},
		{
			name:   "part declaring no type",
			header: textproto.MIMEHeader{},
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var file File
			file.InitFromMultipart(multipartHeader(t, tc.header))
			assert.Equal(t, tc.want, file.ContentType())
		})
	}
}

func TestFileContentTypeOfBytes(t *testing.T) {
	var file File
	file.InitFromBytes([]byte("hello"), "greeting.txt")
	assert.Empty(t, file.ContentType(), "a byte slice declares no media type")
}

func TestFileMetadata(t *testing.T) {
	t.Run("from a multipart part", func(t *testing.T) {
		var file File
		file.InitFromMultipart(multipartHeader(t, textproto.MIMEHeader{}))
		assert.Equal(t, "upload.bin", file.Filename())
		assert.Equal(t, int64(len("payload")), file.FileSize())
	})

	t.Run("from bytes", func(t *testing.T) {
		var file File
		file.InitFromBytes([]byte("hello"), "greeting.txt")
		assert.Equal(t, "greeting.txt", file.Filename())
		assert.Equal(t, int64(len("hello")), file.FileSize())
	})
}

// multipartHeader returns a part parsed out of a real request body, which is the
// state the transport hands one over in.
func multipartHeader(t *testing.T, header textproto.MIMEHeader) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)

	h := textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"; filename="upload.bin"`}}
	for key, values := range header {
		h[key] = values
	}

	part, err := form.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, form.Close())

	reader := multipart.NewReader(body, form.Boundary())
	parsed, err := reader.ReadForm(1 << 20)
	require.NoError(t, err)

	headers := parsed.File["file"]
	require.NotEmpty(t, headers)
	return headers[0]
}
