// Copyright (c) 2004-present Facebook All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package resolver

import (
	"context"
	"testing"

	"github.com/facebookincubator/symphony/graph/ent"
	"github.com/facebookincubator/symphony/graph/ent/equipmentport"
	"github.com/facebookincubator/symphony/graph/ent/equipmentportdefinition"
	"github.com/facebookincubator/symphony/graph/ent/propertytype"
	"github.com/facebookincubator/symphony/graph/graphql/models"
	"github.com/facebookincubator/symphony/graph/viewer/viewertest"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/require"
)

const equipmentType1Port1Name = "typ1_p1"
const equipmentType1Port2Name = "typ1_p2"
const equipmentType2Port1Name = "typ2_p1"
const equipmentType2Port2Name = "typ2_p2"

type portSearchDataModels struct {
	typ1 *ent.EquipmentType
	typ2 *ent.EquipmentType
	e1   *ent.Equipment
	e2   *ent.Equipment
	e3   *ent.Equipment
	e4   *ent.Equipment
	loc1 string
	loc2 string
}

/*
	helper: data now is of type:
	loc1:
		e1(type1)[port: typ1_p1]  <--> e2(type1)[port: typ1_p2]
	loc2:
		e3(type2)[port: typ2_p1]
		e4(type2)[port: typ2_p2]
*/
func preparePortData(ctx context.Context, r *TestResolver) portSearchDataModels {
	mr := r.Mutation()
	locType1, _ := mr.AddLocationType(ctx, models.AddLocationTypeInput{
		Name: "loc_type1",
	})

	loc1, _ := mr.AddLocation(ctx, models.AddLocationInput{
		Name: "loc_inst1",
		Type: locType1.ID,
	})

	loc2, _ := mr.AddLocation(ctx, models.AddLocationInput{
		Name: "loc_inst2",
		Type: locType1.ID,
	})
	ptyp, _ := mr.AddEquipmentPortType(ctx, models.AddEquipmentPortTypeInput{
		Name: "portType1",
		Properties: []*models.PropertyTypeInput{
			{
				Name:        "propStr",
				Type:        "string",
				StringValue: pointer.ToString("t1"),
			},
			{
				Name: "propBool",
				Type: "bool",
			},
			{
				Name: "connected_date",
				Type: models.PropertyKindDate,
			},
		},
	})

	strProp := ptyp.QueryPropertyTypes().Where(propertytype.Name("propStr")).OnlyX(ctx)
	boolProp := ptyp.QueryPropertyTypes().Where(propertytype.Name("propBool")).OnlyX(ctx)
	dateProp := ptyp.QueryPropertyTypes().Where(propertytype.Name("connected_date")).OnlyX(ctx)
	equType1, _ := mr.AddEquipmentType(ctx, models.AddEquipmentTypeInput{
		Name: "eq_type",
		Ports: []*models.EquipmentPortInput{
			{Name: equipmentType1Port1Name, PortTypeID: &ptyp.ID},
			{Name: equipmentType1Port2Name},
		},
	})
	defs1 := equType1.QueryPortDefinitions().AllX(ctx)
	equType2, _ := mr.AddEquipmentType(ctx, models.AddEquipmentTypeInput{
		Name: "eq_type2",
		Ports: []*models.EquipmentPortInput{
			{Name: equipmentType2Port1Name},
			{Name: equipmentType2Port2Name},
		},
	})

	e1, _ := mr.AddEquipment(ctx, models.AddEquipmentInput{
		Name:     "eq_inst1",
		Type:     equType1.ID,
		Location: &loc1.ID,
	})

	def1 := equType1.QueryPortDefinitions().Where(equipmentportdefinition.Name("typ1_p1")).OnlyX(ctx)
	_, _ = mr.EditEquipmentPort(ctx, models.EditEquipmentPortInput{
		Side: &models.LinkSide{
			Equipment: e1.ID,
			Port:      def1.ID,
		},
		Properties: []*models.PropertyInput{
			{
				PropertyTypeID: strProp.ID,
				StringValue:    pointer.ToString("newVal"),
			},
			{
				PropertyTypeID: boolProp.ID,
				BooleanValue:   pointer.ToBool(true),
			},
			{
				PropertyTypeID: dateProp.ID,
				StringValue:    pointer.ToString("1988-03-29"),
			},
		},
	})

	e2, _ := mr.AddEquipment(ctx, models.AddEquipmentInput{
		Name:     "eq_inst2",
		Type:     equType1.ID,
		Location: &loc1.ID,
	})
	e3, _ := mr.AddEquipment(ctx, models.AddEquipmentInput{
		Name:     "eq_inst3",
		Type:     equType2.ID,
		Location: &loc2.ID,
	})
	e4, _ := mr.AddEquipment(ctx, models.AddEquipmentInput{
		Name:     "eq_inst4",
		Type:     equType2.ID,
		Location: &loc2.ID,
	})
	_, _ = mr.AddLink(ctx, models.AddLinkInput{
		Sides: []*models.LinkSide{
			{Equipment: e1.ID, Port: defs1[0].ID},
			{Equipment: e2.ID, Port: defs1[0].ID},
		},
	})
	return portSearchDataModels{
		equType1,
		equType2,
		e1,
		e2,
		e3,
		e4,
		loc1.ID,
		loc2.ID,
	}
}

func TestSearchPortEquipmentName(t *testing.T) {
	r := newTestResolver(t)
	defer r.drv.Close()
	ctx := viewertest.NewContext(r.client)

	data := preparePortData(ctx, r)
	qr := r.Query()
	limit := 100
	all, err := qr.PortSearch(ctx, []*models.PortFilterInput{}, &limit)
	require.NoError(t, err)
	require.Len(t, all.Ports, 8)
	require.Equal(t, all.Count, 8)
	maxDepth := 2
	f1 := models.PortFilterInput{
		FilterType:  models.PortFilterTypePortInstEquipment,
		Operator:    models.FilterOperatorContains,
		StringValue: pointer.ToString(data.e1.Name),
		MaxDepth:    &maxDepth,
	}
	res1, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f1}, &limit)
	require.NoError(t, err)
	ports := res1.Ports
	require.Len(t, ports, 2)
}

func TestSearchPortHasLink(t *testing.T) {
	r := newTestResolver(t)
	defer r.drv.Close()
	ctx := viewertest.NewContext(r.client)

	preparePortData(ctx, r)
	qr := r.Query()
	limit := 100
	all, err := qr.PortSearch(ctx, []*models.PortFilterInput{}, &limit)
	require.NoError(t, err)
	require.Len(t, all.Ports, 8)
	require.Equal(t, all.Count, 8)
	f1 := models.PortFilterInput{
		FilterType: models.PortFilterTypePortInstHasLink,
		Operator:   models.FilterOperatorIs,
		BoolValue:  pointer.ToBool(false),
	}
	res1, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f1}, &limit)
	require.NoError(t, err)
	ports := res1.Ports
	require.Len(t, ports, 6)
}

func TestSearchPortDefinition(t *testing.T) {
	r := newTestResolver(t)
	defer r.drv.Close()
	ctx := viewertest.NewContext(r.client)

	d := preparePortData(ctx, r)

	qr := r.Query()
	limit := 100
	defs := d.typ1.QueryPortDefinitions().AllX(ctx)

	f1 := models.PortFilterInput{
		FilterType: models.PortFilterTypePortDef,
		Operator:   models.FilterOperatorIsOneOf,
		IDSet:      []string{defs[0].ID, defs[1].ID},
	}
	res1, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f1}, &limit)
	require.NoError(t, err)
	ports := res1.Ports
	require.Len(t, ports, 4)
}

func TestSearchPortLocation(t *testing.T) {
	r := newTestResolver(t)
	defer r.drv.Close()
	ctx := viewertest.NewContext(r.client)

	d := preparePortData(ctx, r)
	qr := r.Query()
	limit := 100

	f1 := models.PortFilterInput{
		FilterType: models.PortFilterTypeLocationInst,
		Operator:   models.FilterOperatorIsOneOf,
		IDSet:      []string{d.loc1},
		MaxDepth:   pointer.ToInt(2),
	}
	res1, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f1}, &limit)
	require.NoError(t, err)
	ports := res1.Ports
	require.Len(t, ports, 4)
}

func TestSearchPortProperties(t *testing.T) {
	r := newTestResolver(t)
	defer r.drv.Close()
	ctx := viewertest.NewContext(r.client)

	preparePortData(ctx, r)

	qr := r.Query()
	limit := 100

	f1 := models.PortFilterInput{
		FilterType: models.PortFilterTypeProperty,
		Operator:   models.FilterOperatorIs,
		PropertyValue: &models.PropertyTypeInput{
			Name:        "propStr",
			Type:        models.PropertyKindString,
			StringValue: pointer.ToString("t1"),
		},
	}

	res1, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f1}, &limit)
	require.NoError(t, err)
	ports := res1.Ports
	require.Len(t, ports, 1)

	f2 := models.PortFilterInput{
		FilterType: models.PortFilterTypeProperty,
		Operator:   models.FilterOperatorIs,
		PropertyValue: &models.PropertyTypeInput{
			Name:        "propStr",
			Type:        models.PropertyKindString,
			StringValue: pointer.ToString("newVal"),
		},
	}

	res2, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f2}, &limit)
	require.NoError(t, err)
	ports = res2.Ports
	require.Len(t, ports, 1)

	f3 := models.PortFilterInput{
		FilterType: models.PortFilterTypeProperty,
		Operator:   models.FilterOperatorIs,
		PropertyValue: &models.PropertyTypeInput{
			Name:         "propBool",
			Type:         models.PropertyKindBool,
			BooleanValue: pointer.ToBool(true),
		},
	}

	res3, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f3}, &limit)
	require.NoError(t, err)
	ports = res3.Ports
	require.Len(t, ports, 1)

	f4 := models.PortFilterInput{
		FilterType: models.PortFilterTypeProperty,
		Operator:   models.FilterOperatorIs,
		PropertyValue: &models.PropertyTypeInput{
			Name:         "propBool",
			Type:         models.PropertyKindBool,
			BooleanValue: pointer.ToBool(false),
		},
	}

	res4, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f4}, &limit)
	require.NoError(t, err)
	ports = res4.Ports
	require.Len(t, ports, 0)

	f5 := models.PortFilterInput{
		FilterType: models.PortFilterTypeProperty,
		Operator:   models.FilterOperatorDateLessThan,
		PropertyValue: &models.PropertyTypeInput{
			Name:        "connected_date",
			Type:        models.PropertyKindDate,
			StringValue: pointer.ToString("2019-01-01"),
		},
	}

	res5, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f5}, &limit)
	require.NoError(t, err)
	ports = res5.Ports
	require.Len(t, ports, 1)
}

func TestSearchPortsByService(t *testing.T) {
	r := newTestResolver(t)
	defer r.drv.Close()
	ctx := viewertest.NewContext(r.client)

	data := preparePortData(ctx, r)

	qr, mr := r.Query(), r.Mutation()

	port1, err := data.e1.QueryPorts().Where(equipmentport.HasDefinitionWith(equipmentportdefinition.Name(equipmentType1Port1Name))).Only(ctx)
	require.NoError(t, err)
	port2, err := data.e1.QueryPorts().Where(equipmentport.HasDefinitionWith(equipmentportdefinition.Name(equipmentType1Port2Name))).Only(ctx)
	require.NoError(t, err)
	port3, err := data.e3.QueryPorts().Where(equipmentport.HasDefinitionWith(equipmentportdefinition.Name(equipmentType2Port1Name))).Only(ctx)
	require.NoError(t, err)

	st, _ := mr.AddServiceType(ctx, models.ServiceTypeCreateData{
		Name: "Service Type", HasCustomer: false})

	s1, err := mr.AddService(ctx, models.ServiceCreateData{
		Name:          "Service Instance 1",
		ServiceTypeID: st.ID,
		Status:        pointerToServiceStatus(models.ServiceStatusPending),
	})
	require.NoError(t, err)

	_, err = mr.AddServiceEndpoint(ctx, models.AddServiceEndpointInput{
		ID:     s1.ID,
		PortID: port1.ID,
		Role:   models.ServiceEndpointRoleConsumer,
	})
	require.NoError(t, err)

	s2, err := mr.AddService(ctx, models.ServiceCreateData{
		Name:          "Service Instance 2",
		ServiceTypeID: st.ID,
		Status:        pointerToServiceStatus(models.ServiceStatusPending),
	})
	require.NoError(t, err)
	_, err = mr.AddServiceEndpoint(ctx, models.AddServiceEndpointInput{
		ID:     s2.ID,
		PortID: port1.ID,
		Role:   models.ServiceEndpointRoleConsumer,
	})
	require.NoError(t, err)
	_, err = mr.AddServiceEndpoint(ctx, models.AddServiceEndpointInput{
		ID:     s2.ID,
		PortID: port2.ID,
		Role:   models.ServiceEndpointRoleConsumer,
	})
	require.NoError(t, err)
	_, err = mr.AddServiceEndpoint(ctx, models.AddServiceEndpointInput{
		ID:     s2.ID,
		PortID: port3.ID,
		Role:   models.ServiceEndpointRoleConsumer,
	})
	require.NoError(t, err)

	limit := 100
	all, err := qr.PortSearch(ctx, []*models.PortFilterInput{}, &limit)
	require.NoError(t, err)
	require.Len(t, all.Ports, 8)
	maxDepth := 2

	f1 := models.PortFilterInput{
		FilterType: models.PortFilterTypeServiceInst,
		Operator:   models.FilterOperatorIsOneOf,
		IDSet:      []string{s1.ID},
		MaxDepth:   &maxDepth,
	}
	res1, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f1}, &limit)
	require.NoError(t, err)
	require.Len(t, res1.Ports, 1)
	require.Equal(t, res1.Ports[0].ID, port1.ID)

	f2 := models.PortFilterInput{
		FilterType: models.PortFilterTypeServiceInst,
		Operator:   models.FilterOperatorIsOneOf,
		IDSet:      []string{s2.ID},
		MaxDepth:   &maxDepth,
	}
	res2, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f2}, &limit)
	require.NoError(t, err)
	require.Len(t, res2.Ports, 3)

	f3 := models.PortFilterInput{
		FilterType: models.PortFilterTypeServiceInst,
		Operator:   models.FilterOperatorIsNotOneOf,
		IDSet:      []string{s1.ID},
		MaxDepth:   &maxDepth,
	}
	res3, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f3}, &limit)
	require.NoError(t, err)
	require.Len(t, res3.Ports, 7)

	f4 := models.PortFilterInput{
		FilterType: models.PortFilterTypeServiceInst,
		Operator:   models.FilterOperatorIsNotOneOf,
		IDSet:      []string{s2.ID},
		MaxDepth:   &maxDepth,
	}
	res4, err := qr.PortSearch(ctx, []*models.PortFilterInput{&f4}, &limit)
	require.NoError(t, err)
	require.Len(t, res4.Ports, 5)
}
