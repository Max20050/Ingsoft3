package datasource

// SystemField is one of the predefined fields a data source's detected fields
// can be mapped onto. This starter catalog covers the common fields of a
// business document (invoice, receipt, order); it isn't sourced from any
// other domain in this codebase, since none defines one yet — see issue #41.
type SystemField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// SystemFields is the fixed list of targets a field mapping can point to.
var SystemFields = []SystemField{
	{Key: "document_number", Label: "Document number"},
	{Key: "document_date", Label: "Document date"},
	{Key: "due_date", Label: "Due date"},
	{Key: "customer_name", Label: "Customer name"},
	{Key: "customer_email", Label: "Customer email"},
	{Key: "vendor_name", Label: "Vendor name"},
	{Key: "description", Label: "Description"},
	{Key: "quantity", Label: "Quantity"},
	{Key: "unit_price", Label: "Unit price"},
	{Key: "subtotal", Label: "Subtotal"},
	{Key: "tax_amount", Label: "Tax amount"},
	{Key: "total_amount", Label: "Total amount"},
	{Key: "currency", Label: "Currency"},
	{Key: "status", Label: "Status"},
}

// isValidSystemField reports whether key names one of SystemFields.
func isValidSystemField(key string) bool {
	for _, f := range SystemFields {
		if f.Key == key {
			return true
		}
	}
	return false
}
