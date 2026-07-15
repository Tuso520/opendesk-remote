package artifacts

type Artifact struct {
	Platform  string
	Filename  string
	LocalPath string
	SHA256    string
	SizeBytes int64
}
