package signing

type Config struct {
	Windows string `json:"windows"`
	MacOS   string `json:"macos"`
	Android string `json:"android"`
	IOS     string `json:"ios"`
}
