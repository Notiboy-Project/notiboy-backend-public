package utilities

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base32"
	"encoding/hex"
	"strings"
	"text/template"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

func DBMultiValuePlaceholders(n int) string {
	var b strings.Builder
	b.WriteString("(")
	b.WriteString(strings.TrimSuffix(strings.Repeat("?,", n), ","))
	b.WriteString("),")
	return strings.TrimSuffix(b.String(), ",")
}

// function produces hashed string using md5
func Encrypt(str string) string {
	hasher := sha512.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

func GenerateRandomToken() (string, error) {
	// Generate a random sequence of 3 bytes
	randBytes := make([]byte, 3)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", err
	}

	// Encode the random bytes using Base32 encoding
	token := base32.StdEncoding.EncodeToString(randBytes)[:6]

	return token, nil
}

func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func SliceToMap(sl []string) map[string]bool {
	m := make(map[string]bool)

	for _, each := range sl {
		m[each] = true
	}

	return m
}

func ToDate(t time.Time) string {
	// changing the time format to "yyyy-mm-dd"
	return t.Format("2006-01-02")
}

func TimeStringToDate(s string) string {
	return ToDate(TimeStringToTime(s))
}

func TimeStringToTime(s string) time.Time {
	// changing the time format to "yyyy-mm-dd"
	layout := "2006-01-02 15:04:05 -0700 MST"
	t, err := time.Parse(layout, s)
	if err != nil {
		logrus.WithError(err).Errorf("TimeStringToTime failed for %s", s)
		return time.Time{}
	}

	return t
}

func TimeStringToRFC3339Time(s string) time.Time {
	// changing the time format to "yyyy-mm-dd"
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		logrus.WithError(err).Errorf("TimeStringToRFC3339Time failed for %s", s)
		return time.Time{}
	}

	return t
}

func DateStringToTime(s string) time.Time {
	// changing the time format to "yyyy-mm-dd"
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		logrus.WithError(err).Errorf("DateStringToTime failed for %s", s)
		return time.Time{}
	}

	return t
}
func TimeNow() time.Time {
	return time.Now().UTC()
}

func UnixTimeString() string {
	return cast.ToString(TimeNow().Unix())
}

func UnixTime() int64 {
	return TimeNow().Unix()
}

func DateNow() string {
	return ToDate(TimeNow())
}

func TemplateRendering(tmpl *template.Template, data any) (*bytes.Buffer, error) {
	body := new(bytes.Buffer)

	// Execute the template with the data and store the result in a buffer
	err := tmpl.Execute(body, data)
	if err != nil {
		return body, err
	}
	return body, err
}

func BeginningOfMonth(date time.Time) time.Time {
	return date.AddDate(0, 0, -date.Day()+1)
}

func EndOfMonth(date time.Time) time.Time {
	return date.AddDate(0, 1, -date.Day())
}
