package penelope

func uriFromParts(did string, collection string, rkey string) string {
	return "at://" + did + "/" + collection + "/" + rkey
}
