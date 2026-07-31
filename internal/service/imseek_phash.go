package service

import (
	"bytes"

	"github.com/wwwangzilin/LotsACG/internal/pkg/mediatool"
)

func getPhashFromBytes(imageBytes []byte) (string, error) {
	return mediatool.GetImagePhashFromReader(bytes.NewReader(imageBytes))
}
