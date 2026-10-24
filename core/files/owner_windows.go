package files

import "os"

// owner is empty: Windows files have no Unix owner.
func owner(os.FileInfo) string { return "" }
