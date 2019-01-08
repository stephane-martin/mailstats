package extractors

import set "github.com/deckarep/golang-set"

var exeTypes = set.NewSetWith(
	"application/x-dosexec",
	"application/x-msdownload",
	"application/exe",
	"application/x-exe",
	"application/dos-exe",
	"vms/exe",
	"application/x-winexe",
	"application/msdos-windows",
	"application/x-msdos-program",
	"application/x-executable",
	"application/vnd.microsoft.portable-executable",
)

func IsExecutable(mimetype string) bool {
	return exeTypes.Contains(mimetype)
}
