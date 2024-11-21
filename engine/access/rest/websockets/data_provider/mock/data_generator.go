package mock

type DataGenerator struct {
	data []interface{}
}

func NewDataGenerator(data []interface{}) *DataGenerator {
	return &DataGenerator{
		data: data,
	}
}

func (d *DataGenerator) Next() interface{} {
	if len(d.data) == 0 {
		return nil
	}

	next := d.data[len(d.data)-1]
	d.data = d.data[:len(d.data)-1]
	return next
}
