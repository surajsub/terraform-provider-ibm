package ibm

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/IBM/go-sdk-core/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

const (
	rID          = "route_id"
	rDestination = "destination"
	rAction      = "action"
	rNextHop     = "next_hop"
	rName        = "name"
	rZone        = "zone"
)

func resourceIBMISVPCRoutingTableRoute() *schema.Resource {
	return &schema.Resource{
		Create:   resourceIBMISVPCRoutingTableRouteCreate,
		Read:     resourceIBMISVPCRoutingTableRouteRead,
		Update:   resourceIBMISVPCRoutingTableRouteUpdate,
		Delete:   resourceIBMISVPCRoutingTableRouteDelete,
		Exists:   resourceIBMISVPCRoutingTableRouteExists,
		Importer: &schema.ResourceImporter{},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			rtID: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The routing table identifier.",
			},
			rtVpcID: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The VPC identifier.",
			},
			rDestination: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The destination of the route.",
			},
			rZone: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The zone to apply the route to. Traffic from subnets in this zone will be subject to this route.",
			},
			rAction: {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Default:      "deliver",
				Description:  "The action to perform with a packet matching the route.",
				ValidateFunc: InvokeValidator("ibm_is_vpc_routing_table_route", rAction),
			},
			rNextHop: {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "If action is deliver, the next hop that packets will be delivered to. For other action values, its address will be 0.0.0.0.",
			},
			rName: {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     false,
				Computed:     true,
				Description:  "The user-defined name for this route.",
				ValidateFunc: InvokeValidator("ibm_is_vpc_routing_table_route", rName),
			},
			rID: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The routing table route identifier.",
			},
			rtHref: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Routing table route Href",
			},
			rtCreateAt: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Routing table route Created At",
			},
			rtLifecycleState: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Routing table route Lifecycle State",
			},
			rtOrigin: {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The origin of this route.",
			},
		},
	}
}

func resourceIBMISVPCRoutingTableRouteValidator() *ResourceValidator {

	validateSchema := make([]ValidateSchema, 2)
	actionAllowedValues := "delegate, delegate_vpc, deliver, drop"

	validateSchema = append(validateSchema,
		ValidateSchema{
			Identifier:                 rtName,
			ValidateFunctionIdentifier: ValidateRegexpLen,
			Type:                       TypeString,
			Required:                   false,
			Regexp:                     `^([a-z]|[a-z][-a-z0-9]*[a-z0-9])$`,
			MinValueLength:             1,
			MaxValueLength:             63})

	validateSchema = append(validateSchema,
		ValidateSchema{
			Identifier:                 rAction,
			ValidateFunctionIdentifier: ValidateAllowedStringValue,
			Type:                       TypeString,
			Required:                   false,
			AllowedValues:              actionAllowedValues})

	ibmVPCRoutingTableRouteValidator := ResourceValidator{ResourceName: "ibm_is_vpc_routing_table_route", Schema: validateSchema}
	return &ibmVPCRoutingTableRouteValidator
}

func resourceIBMISVPCRoutingTableRouteCreate(d *schema.ResourceData, meta interface{}) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}

	vpcID := d.Get(rtVpcID).(string)
	tableID := d.Get(rtID).(string)
	action := d.Get(rAction).(string)
	nextHop := d.Get(rNextHop).(string)
	routeName := d.Get(rName).(string)
	destination := d.Get(rDestination).(string)
	zone := d.Get(rZone).(string)
	z := &vpcv1.ZoneIdentityByName{
		Name: core.StringPtr(zone),
	}

	nh := &vpcv1.RouteNextHopPrototypeRouteNextHopIP{
		Address: core.StringPtr(nextHop),
	}

	createVpcRoutingTableRouteOptions := sess.NewCreateVPCRoutingTableRouteOptions(vpcID, tableID, destination, nh, z)
	createVpcRoutingTableRouteOptions.SetName(routeName)
	createVpcRoutingTableRouteOptions.SetZone(z)
	createVpcRoutingTableRouteOptions.SetDestination(destination)
	createVpcRoutingTableRouteOptions.SetNextHop(nh)
	createVpcRoutingTableRouteOptions.SetAction(action)

	route, response, err := sess.CreateVPCRoutingTableRoute(createVpcRoutingTableRouteOptions)
	if err != nil {
		log.Printf("[DEBUG] Create VPC Routing table route err %s\n%s", err, response)
		return err
	}

	d.SetId(fmt.Sprintf("%s/%s/%s", vpcID, tableID, *route.ID))
	d.Set(rID, *route.ID)
	return resourceIBMISVPCRoutingTableRouteRead(d, meta)
}

func resourceIBMISVPCRoutingTableRouteRead(d *schema.ResourceData, meta interface{}) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}

	idSet := strings.Split(d.Id(), "/")
	getVpcRoutingTableRouteOptions := sess.NewGetVPCRoutingTableRouteOptions(idSet[0], idSet[1], idSet[2])
	route, response, err := sess.GetVPCRoutingTableRoute(getVpcRoutingTableRouteOptions)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error Getting VPC Routing table route: %s\n%s", err, response)
	}

	d.Set(rID, *route.ID)
	d.Set(rName, *route.Name)
	d.Set(rDestination, *route.Destination)
	if route.NextHop != nil {
		nexthop := route.NextHop.(*vpcv1.RouteNextHop)
		//route[isRoutingTableRouteNexthop] = *nexthop.Address
		//nh := response.NextHop.(map[string]interface{})
		//nh := *response.NextHop.(vpcv1.RouteNextHopPrototype)
		d.Set(rNextHop, *nexthop.Address)
	}
	if route.Zone != nil {
		d.Set(rZone, *route.Zone.Name)
	}
	d.Set(rtHref, route.Href)
	d.Set(rtLifecycleState, route.LifecycleState)
	d.Set(rtCreateAt, route.CreatedAt.String())

	return nil
}

func resourceIBMISVPCRoutingTableRouteUpdate(d *schema.ResourceData, meta interface{}) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}

	idSet := strings.Split(d.Id(), "/")
	if d.HasChange(rName) {
		routePatch := make(map[string]interface{})
		updateVpcRoutingTableRouteOptions := sess.NewUpdateVPCRoutingTableRouteOptions(idSet[0], idSet[1], idSet[2], routePatch)

		// Construct an instance of the RoutePatch model
		routePatchModel := new(vpcv1.RoutePatch)
		name := d.Get(rName).(string)
		routePatchModel.Name = &name
		routePatchModelAsPatch, patchErr := routePatchModel.AsPatch()

		if patchErr != nil {
			return fmt.Errorf("Error calling asPatch for VPC Routing Table Route Patch: %s", patchErr)
		}

		updateVpcRoutingTableRouteOptions.RoutePatch = routePatchModelAsPatch
		_, response, err := sess.UpdateVPCRoutingTableRoute(updateVpcRoutingTableRouteOptions)
		if err != nil {
			log.Printf("[DEBUG] Update VPC Routing table route err %s\n%s", err, response)
			return err
		}
	}

	return resourceIBMISVPCRoutingTableRouteRead(d, meta)
}

func resourceIBMISVPCRoutingTableRouteDelete(d *schema.ResourceData, meta interface{}) error {
	sess, err := vpcClient(meta)
	if err != nil {
		return err
	}

	idSet := strings.Split(d.Id(), "/")
	deleteVpcRoutingTableRouteOptions := sess.NewDeleteVPCRoutingTableRouteOptions(idSet[0], idSet[1], idSet[2])
	response, err := sess.DeleteVPCRoutingTableRoute(deleteVpcRoutingTableRouteOptions)
	if err != nil && response.StatusCode != 404 {
		log.Printf("Error deleting VPC Routing table route : %s", response)
		return err
	}

	d.SetId("")
	return nil
}

func resourceIBMISVPCRoutingTableRouteExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	sess, err := vpcClient(meta)
	if err != nil {
		return false, err
	}

	idSet := strings.Split(d.Id(), "/")
	getVpcRoutingTableRouteOptions := sess.NewGetVPCRoutingTableRouteOptions(idSet[0], idSet[1], idSet[2])
	_, response, err := sess.GetVPCRoutingTableRoute(getVpcRoutingTableRouteOptions)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			d.SetId("")
			return false, nil
		}
		return false, fmt.Errorf("Error Getting VPC Routing table route : %s\n%s", err, response)
	}
	return true, nil
}