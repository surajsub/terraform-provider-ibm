/* IBM Confidential
*  Object Code Only Source Materials
*  5747-SM3
*  (c) Copyright IBM Corp. 2017,2021
*
*  The source code for this program is not published or otherwise divested
*  of its trade secrets, irrespective of what has been deposited with the
*  U.S. Copyright Office. */

package ibm

import (
	"fmt"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_volumes"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/IBM-Cloud/bluemix-go/bmxerror"
	st "github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/helpers"
)

const (
	/* Power Volume creation depends on response from PowerVC */
	volPostTimeOut   = 180 * time.Second
	volGetTimeOut    = 180 * time.Second
	volDeleteTimeOut = 180 * time.Second
)

func resourceIBMPIVolume() *schema.Resource {
	return &schema.Resource{
		Create:   resourceIBMPIVolumeCreate,
		Read:     resourceIBMPIVolumeRead,
		Update:   resourceIBMPIVolumeUpdate,
		Delete:   resourceIBMPIVolumeDelete,
		Exists:   resourceIBMPIVolumeExists,
		Importer: &schema.ResourceImporter{},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},

		Schema: map[string]*schema.Schema{

			helpers.PIVolumeName: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Volume Name to create",
			},

			helpers.PIVolumeShareable: {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Flag to indicate if the volume can be shared across multiple instances?",
			},
			helpers.PIVolumeSize: {
				Type:        schema.TypeFloat,
				Required:    true,
				Description: "Size of the volume in GB",
			},
			helpers.PIVolumeType: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateAllowedStringValue([]string{"tier1", "tier3"}),
				Description:  "Volume type",
			},

			helpers.PICloudInstanceId: {
				Type:        schema.TypeString,
				Required:    true,
				Description: " Cloud Instance ID - This is the service_instance_id.",
			},

			helpers.PIInstanceName: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Instance Name to preserve the Volume Affinity",
				//ConflictsWith: []string{"affinity_volume"},
			},

			"volume_count": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
				Description: "Count of the volumes: Note - Every volume will be of the same size",
			},

			helpers.PIAffinityVolume: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Volume id that we need affinity to",
				//ConflictsWith: []string{helpers.PIInstanceName},
			},

			helpers.PIAffinityPolicy: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Volume Affinity",
			},

			// Computed Attributes

			"volume_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Volume ID",
			},

			"volume_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Volume status",
			},

			"delete_on_termination": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Should the volume be deleted during termination",
			},
			"wwn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "WWN Of the volume",
			},
			"pool_volume_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Volume Type-Represents the pool",
			},
		},
	}
}

func resourceIBMPIVolumeCreate(d *schema.ResourceData, meta interface{}) error {
	sess, err := meta.(ClientSession).IBMPISession()
	if err != nil {
		return err
	}

	var (
		instanceid      string
		affinity_volume string
		affinity_policy string
		volume_count    int
	)

	name := d.Get(helpers.PIVolumeName).(string)
	volType := d.Get(helpers.PIVolumeType).(string)
	size := int64(d.Get(helpers.PIVolumeSize).(float64))
	powerinstanceid := d.Get(helpers.PICloudInstanceId).(string)

	log.Printf("instance name is %s and policy is %s", d.Get(helpers.PIInstanceName), d.Get(helpers.PIAffinityPolicy))

	if d.Get(helpers.PIInstanceName) != "" && d.Get(helpers.PIAffinityPolicy) == "" {
		return fmt.Errorf("if hostname is provided, the affinity policy also should be provided")
	}

	var shared bool
	if v, ok := d.GetOk(helpers.PIVolumeShareable); ok {
		shared = v.(bool)
	}

	if v, ok := d.GetOk("volume_count"); ok {
		volume_count = v.(int)
	}
	if v, ok := d.GetOk(helpers.PIInstanceName); ok {
		instanceid = v.(string)
	}
	if v, ok := d.GetOk("affinity_volume"); ok {
		affinity_volume = v.(string)
	}

	if v, ok := d.GetOk(helpers.PIAffinityPolicy); ok {
		affinity_policy = v.(string)
	}

	client := st.NewIBMPIVolumeClient(sess, powerinstanceid)

	vol2body := &models.MultiVolumesCreate{
		AffinityPVMInstance: &instanceid,
		AffinityPolicy:      nil,
		AffinityVolume:      nil,
		DiskType:            volType,
		Name:                &name,
		Shareable:           &shared,
		Size:                &size,
		VolumePool:          "",
	}

	if volume_count > 1 {
		vol2body.Count = int64(volume_count)
	} else {
		vol2body.Count = 1
	}

	if instanceid != "" {
		vol2body.AffinityPVMInstance = &instanceid
	}
	if affinity_volume != "" {
		vol2body.AffinityVolume = &affinity_volume
	}

	if affinity_policy != "" {
		vol2body.AffinityPolicy = &affinity_policy
		vol2body.AffinityPVMInstance = &instanceid
	}

	vol2, err := client.CreateVolumeV2(&p_cloud_volumes.PcloudV2VolumesPostParams{
		Body: vol2body,
	}, powerinstanceid, helpers.PICreateTimeOut)
	//vol, err := client.Create(name, size, volType, shared, powerinstanceid, volPostTimeOut)

	if err != nil {
		return fmt.Errorf("Failed to Create the volume %v", err)
	}

	volumeid := *vol2.Volumes[0].VolumeID

	d.SetId(fmt.Sprintf("%s/%s", powerinstanceid, volumeid))
	_, err = isWaitForIBMPIVolumeAvailable(client, volumeid, powerinstanceid, d.Timeout(schema.TimeoutCreate))
	if err != nil {
		return err
	}

	return resourceIBMPIVolumeRead(d, meta)
}

func resourceIBMPIVolumeRead(d *schema.ResourceData, meta interface{}) error {
	sess, _ := meta.(ClientSession).IBMPISession()
	parts, err := idParts(d.Id())
	if err != nil {
		return err
	}
	powerinstanceid := parts[0]
	client := st.NewIBMPIVolumeClient(sess, powerinstanceid)

	vol, err := client.Get(parts[1], powerinstanceid, volGetTimeOut)
	if err != nil {
		return fmt.Errorf("Failed to get the volume %v", err)

	}
	d.Set(helpers.PIVolumeName, vol.Name)
	d.Set(helpers.PIVolumeSize, vol.Size)
	if &vol.Shareable != nil {
		d.Set(helpers.PIVolumeShareable, vol.Shareable)
	}
	d.Set(helpers.PIVolumeType, vol.DiskType)
	if &vol.State != nil {
		d.Set("volume_status", vol.State)
	}
	if &vol.VolumeID != nil {
		d.Set("volume_id", vol.VolumeID)
	}
	if &vol.DeleteOnTermination != nil {
		d.Set("delete_on_termination", vol.DeleteOnTermination)
	}
	if &vol.Wwn != nil {
		d.Set("wwn", vol.Wwn)
	}

	if &vol.VolumeType != nil {
		d.Set("pool_volume_type", vol.VolumeType)
	}
	d.Set(helpers.PICloudInstanceId, powerinstanceid)

	return nil
}

func resourceIBMPIVolumeUpdate(d *schema.ResourceData, meta interface{}) error {
	sess, _ := meta.(ClientSession).IBMPISession()
	parts, err := idParts(d.Id())
	if err != nil {
		return err
	}
	powerinstanceid := parts[0]
	client := st.NewIBMPIVolumeClient(sess, powerinstanceid)
	name := d.Get(helpers.PIVolumeName).(string)
	size := float64(d.Get(helpers.PIVolumeSize).(float64))
	var shareable bool
	if v, ok := d.GetOk(helpers.PIVolumeShareable); ok {
		shareable = v.(bool)
	}
	volrequest, err := client.Update(parts[1], name, size, shareable, powerinstanceid, volPostTimeOut)
	if err != nil {
		return err
	}
	_, err = isWaitForIBMPIVolumeAvailable(client, *volrequest.VolumeID, powerinstanceid, d.Timeout(schema.TimeoutUpdate))
	if err != nil {
		return err
	}

	return resourceIBMPIVolumeRead(d, meta)
}

func resourceIBMPIVolumeDelete(d *schema.ResourceData, meta interface{}) error {

	sess, _ := meta.(ClientSession).IBMPISession()
	parts, err := idParts(d.Id())
	if err != nil {
		return err
	}
	powerinstanceid := parts[0]

	client := st.NewIBMPIVolumeClient(sess, powerinstanceid)
	voldeleteErr := client.Delete(parts[1], powerinstanceid, deleteTimeOut)
	if voldeleteErr != nil {
		return voldeleteErr
	}
	_, err = isWaitForIBMPIVolumeDeleted(client, parts[1], powerinstanceid, d.Timeout(schema.TimeoutDelete))
	if err != nil {
		return err
	}
	d.SetId("")
	return nil
}
func resourceIBMPIVolumeExists(d *schema.ResourceData, meta interface{}) (bool, error) {

	sess, err := meta.(ClientSession).IBMPISession()
	if err != nil {
		return false, err
	}
	parts, err := idParts(d.Id())
	if err != nil {
		return false, err
	}

	powerinstanceid := parts[0]
	client := st.NewIBMPIVolumeClient(sess, powerinstanceid)

	vol, err := client.Get(parts[1], powerinstanceid, getTimeOut)
	if err != nil {
		if apiErr, ok := err.(bmxerror.RequestFailure); ok {
			if apiErr.StatusCode() == 404 {
				return false, nil
			}
		}
		return false, fmt.Errorf("Error communicating with the API: %s", err)
	}

	log.Printf("Calling the existing function.. %s", *(vol.VolumeID))

	volumeid := *vol.VolumeID
	return volumeid == parts[1], nil
}

func isWaitForIBMPIVolumeAvailable(client *st.IBMPIVolumeClient, id, powerinstanceid string, timeout time.Duration) (interface{}, error) {
	log.Printf("Waiting for Volume (%s) to be available.", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"retry", helpers.PIVolumeProvisioning},
		Target:     []string{helpers.PIVolumeProvisioningDone},
		Refresh:    isIBMPIVolumeRefreshFunc(client, id, powerinstanceid),
		Delay:      10 * time.Second,
		MinTimeout: 2 * time.Minute,
		Timeout:    30 * time.Minute,
	}

	return stateConf.WaitForState()
}

func isIBMPIVolumeRefreshFunc(client *st.IBMPIVolumeClient, id, powerinstanceid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		vol, err := client.Get(id, powerinstanceid, volGetTimeOut)
		if err != nil {
			return nil, "", err
		}

		if vol.State == "available" {
			return vol, helpers.PIVolumeProvisioningDone, nil
		}

		return vol, helpers.PIVolumeProvisioning, nil
	}
}

func isWaitForIBMPIVolumeDeleted(client *st.IBMPIVolumeClient, id, powerinstanceid string, timeout time.Duration) (interface{}, error) {
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"deleting", helpers.PIVolumeProvisioning},
		Target:     []string{"deleted"},
		Refresh:    isIBMPIVolumeDeleteRefreshFunc(client, id, powerinstanceid),
		Delay:      10 * time.Second,
		MinTimeout: 2 * time.Minute,
		Timeout:    30 * time.Minute,
	}
	return stateConf.WaitForState()
}

func isIBMPIVolumeDeleteRefreshFunc(client *st.IBMPIVolumeClient, id, powerinstanceid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		vol, err := client.Get(id, powerinstanceid, volGetTimeOut)
		if err != nil {
			if strings.Contains(err.Error(), "Resource not found") {
				return vol, "deleted", nil
			}
			return nil, "", err
		}
		if vol == nil {
			return vol, "deleted", nil
		}
		return vol, "deleting", nil
	}
}
