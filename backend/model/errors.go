package model

type ErrNotFound string

func (e ErrNotFound) Error() string {
	return string(e)
}

type ErrBadRequest string

func (e ErrBadRequest) Error() string {
	return string(e)
}
