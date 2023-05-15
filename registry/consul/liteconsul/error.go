package liteconsul

type Error struct {
	Code    int
	Content string
}

func (e *Error) Error() string {
	return e.Content
}

func IsNotFound(err error) bool {
	if e, ok := err.(*Error); ok && e.Code == 404 {
		return true
	}
	return false
}
