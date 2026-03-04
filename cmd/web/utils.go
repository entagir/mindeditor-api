package main

func getWebId(id int) string {
	webIdAsString, _ := h.Encode([]int{0, id})
	return webIdAsString
}

func getIdFromWebId(webIdAsString string) uint {
	id, err := h.DecodeWithError(webIdAsString)

	if err != nil {
		return 0
	}
	return uint(id[1])
}
