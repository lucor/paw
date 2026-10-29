// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package haveibeenpwned

import (
	"bytes"
	"context"
	"crypto/sha1"
	"io"
	"net/http"
	"testing"
)

const responseBody = `1CB7055517A54D1B0F1847EB84904E69438:2
1CC93AEF7B58A1B631CB55BF3A3A3750285:3
1D2DA4053E34E76F6576ED1DA63134B5E2A:2
1D72CD07550416C216D8AD296BF5C0AE8E0:10
1DE027315DE413921A63F1700938AF80965:1
1E2AAA439972480CEC7F16C795BBB429371:0
1E2AAA439972480CEC7F16C795BBB429372:1
1E2AAA439972480CEC7F16C795BBB429373:0
1E2AAA439972480CEC7F16C795BBB429374:0
1E2AAA439972480CEC7F16C795BBB429375:0
1E3687A61BFCE35F69B7408158101C8E414:1
1E4C9B93F3F0682250B6CF8331B7EE68FD8:3861493
1F15311317129463049803B0F5AE31A31C4:1
1F2B668E8AABEF1C59E9EC6F82E3F3CD786:1
2028CB7ABE16047F9FFB0699E25655236E0:1
20597F5AC10A2F67701B4AD1D3A09F72250:3`

type mockClient struct {
	request *http.Request
}

func (c *mockClient) Do(req *http.Request) (*http.Response, error) {
	c.request = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(responseBody))),
	}, nil
}

func TestHashLookupSendsOnlyPrefix(t *testing.T) {
	client := &mockClient{}
	hash := sha1.Sum([]byte("password"))
	exposed, count, err := hibp(context.Background(), client, hash)
	if err != nil || !exposed || count != 3861493 {
		t.Fatalf("got exposed=%v count=%d err=%v", exposed, count, err)
	}
	if got := client.request.URL.String(); got != "https://api.pwnedpasswords.com/range/5BAA6" {
		t.Fatalf("unexpected lookup URL: %s", got)
	}
	if client.request.Body != nil || client.request.Header.Get("Add-Padding") != "true" {
		t.Fatal("expected a bodyless, padded range request")
	}
}

func Test_pwned(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		want      bool
		wantCount int
		wantErr   bool
	}{
		{
			name:      "password pwned",
			password:  "password", // SHA1: 5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8
			want:      true,
			wantCount: 3861493,
			wantErr:   false,
		},
		{
			name:      "not pwned password",
			password:  ":-)",
			want:      false,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := hibp(context.Background(), &mockClient{}, sha1.Sum([]byte(tt.password)))
			if (err != nil) != tt.wantErr {
				t.Errorf("pwned() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("pwned() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.wantCount {
				t.Errorf("pwned() got1 = %v, want %v", got1, tt.wantCount)
			}
		})
	}
}

type responseClient struct {
	status int
	body   string
}

func (c responseClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: c.status, Body: io.NopCloser(bytes.NewBufferString(c.body))}, nil
}

func TestLookupFailureIsNotASuccessfulCheck(t *testing.T) {
	for _, client := range []responseClient{
		{http.StatusServiceUnavailable, "service unavailable"},
		{http.StatusTooManyRequests, "rate limited"},
		{http.StatusOK, ""},
		{http.StatusOK, "invalid response"},
	} {
		if _, _, err := hibp(context.Background(), client, sha1.Sum([]byte("password"))); err == nil {
			t.Fatalf("expected error for status %d, body %q", client.status, client.body)
		}
	}
}

func TestPaddingMatchIsNotExposed(t *testing.T) {
	client := responseClient{http.StatusOK, "1E4C9B93F3F0682250B6CF8331B7EE68FD8:0"}
	exposed, count, err := hibp(context.Background(), client, sha1.Sum([]byte("password")))
	if err != nil || exposed || count != 0 {
		t.Fatalf("got exposed=%v count=%d err=%v", exposed, count, err)
	}
}
