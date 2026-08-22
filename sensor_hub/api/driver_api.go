package api

import (
	"example/sensorHub/drivers"
	gen "example/sensorHub/gen"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) ListDrivers(c *gin.Context, params gen.ListDriversParams) {
	allDrivers := drivers.All()

	result := make([]gen.DriverInfo, 0, len(allDrivers))
	for _, d := range allDrivers {
		if params.Type != nil {
			switch *params.Type {
			case gen.Pull:
				if _, ok := d.(drivers.PullDriver); !ok {
					continue
				}
			case gen.Push:
				if _, ok := d.(drivers.PushDriver); !ok {
					continue
				}
			}
		}
		result = append(result, driverToGenInfo(d))
	}
	c.IndentedJSON(http.StatusOK, result)
}

func driverToGenInfo(d drivers.SensorDriver) gen.DriverInfo {
	mtNames := make([]string, 0)
	for _, mt := range d.SupportedMeasurementTypes() {
		mtNames = append(mtNames, mt.Name)
	}
	desc := d.Description()
	fields := d.ConfigFields()
	configFields := make([]gen.ConfigFieldSpec, len(fields))
	for i, f := range fields {
		configFields[i] = convertConfigFieldSpec(f)
	}
	return gen.DriverInfo{
		Type:                      d.Type(),
		DisplayName:               d.DisplayName(),
		Description:               &desc,
		SupportedMeasurementTypes: &mtNames,
		ConfigFields:              configFields,
	}
}

func convertConfigFieldSpec(f drivers.ConfigFieldSpec) gen.ConfigFieldSpec {
	spec := gen.ConfigFieldSpec{
		Key:       f.Key,
		Label:     f.Label,
		Required:  f.Required,
		Sensitive: f.Sensitive,
	}
	spec.Description = &f.Description
	if f.Default != "" {
		spec.Default = &f.Default
	}
	return spec
}
