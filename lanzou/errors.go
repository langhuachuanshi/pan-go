package lanzou

import "errors"

var (
	ErrNotLoggedIn    = errors.New("lanzou: not logged in")
	ErrFileExpired    = errors.New("lanzou: file expired or deleted")
	ErrPasswordWrong  = errors.New("lanzou: wrong password")
	ErrFileSizeLimit  = errors.New("lanzou: file size exceeds limit")
	ErrInvalidURL     = errors.New("lanzou: invalid lanzou url")
	ErrExtractFailed  = errors.New("lanzou: failed to extract data from page")
	ErrUploadFailed   = errors.New("lanzou: upload failed")
	ErrDownloadFailed = errors.New("lanzou: download failed")
	ErrAPIError       = errors.New("lanzou: api error")
)
