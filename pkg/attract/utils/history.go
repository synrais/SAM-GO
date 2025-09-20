package attract

var currentIndex int = -1

// Play appends a game to history.
func Play(path string) {
	hist := GetList("History.txt")
	hist = append(hist, path)
	SetList("History.txt", hist)
	currentIndex = len(hist) - 1
}

// Next moves forward in history.
func Next() (string, bool) {
	hist := GetList("History.txt")
	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		return hist[currentIndex], true
	}
	return "", false
}

// Back moves backward in history.
func Back() (string, bool) {
	hist := GetList("History.txt")
	if currentIndex > 0 {
		currentIndex--
		return hist[currentIndex], true
	}
	return "", false
}

func PlayNext() (string, bool) { return Next() }
func PlayBack() (string, bool) { return Back() }
