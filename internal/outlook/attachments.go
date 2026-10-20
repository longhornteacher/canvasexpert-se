package outlook

// MAPI attachment properties are built separately so CID/body agreement can be
// verified without driving Outlook or sending mail.
type attachmentProperty struct {
	Schema string
	Value  any
}

func attachmentProperties(a Attachment) []attachmentProperty {
	const prefix = "http://schemas.microsoft.com/mapi/proptag/"
	var p []attachmentProperty
	if a.ContentType != "" {
		p = append(p, attachmentProperty{prefix + "0x370E001F", a.ContentType})
	}
	if a.ContentID != "" {
		p = append(p, attachmentProperty{prefix + "0x3712001F", a.ContentID}, attachmentProperty{prefix + "0x7FFE000B", true})
	}
	return p
}
