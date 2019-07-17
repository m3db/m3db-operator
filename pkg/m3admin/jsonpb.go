// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package m3admin

import (
	"io"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

// JSONPBUnmarshal unmarshals a JSON protobuf message, allowing
// for backwards compatible changes by allowing unknown fields to
// exist when unmarshalling response (new fields will get added
// and need to be able to be unmarshalled by an old operator
// calling endpoints on a new coordinator or DB node).
func JSONPBUnmarshal(r io.Reader, msg proto.Message) error {
	unmarshaller := &jsonpb.Unmarshaler{AllowUnknownFields: true}
	return unmarshaller.Unmarshal(r, msg)
}

// JSONPBMarshal marshals a JSON protobuf message, just using
// defaults for the marshaller currently.
func JSONPBMarshal(w io.Writer, msg proto.Message) error {
	marshaller := &jsonpb.Marshaler{}
	return marshaller.Marshal(w, msg)
}
