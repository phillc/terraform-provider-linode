package linode

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/linode/linodego"
)

func resourceLinodeFirewallRule() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"label": {
				Type:        schema.TypeString,
				Description: `Used to identify this rule. For display purposes only.`,
				Required:    true,
			},
			"action": {
				Type:        schema.TypeString,
				Description: `Controls whether traffic is accepted or dropped by this rule. Overrides the Firewall’s inbound_policy if this is an inbound rule, or the outbound_policy if this is an outbound rule.`,
				Required:    true,
			},
			"ports": {
				Type:        schema.TypeString,
				Description: `A string representation of ports and/or port ranges (i.e. "443" or "80-90, 91").`,
				Optional:    true,
			},
			"protocol": {
				Type:         schema.TypeString,
				Description:  "The network protocol this rule controls.",
				ValidateFunc: validation.StringInSlice([]string{"TCP", "UDP", "ICMP"}, false),
				Required:     true,
			},
			"ipv4": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A list of IP addresses, CIDR blocks, or 0.0.0.0/0 (to allow all) this rule applies to.",
				Optional:    true,
			},
			"ipv6": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A list of IPv6 addresses or networks this rule applies to.",
				MinItems:    1,
				Optional:    true,
			},
		},
	}
}

func resourceLinodeFirewall() *schema.Resource {
	return &schema.Resource{
		Create: resourceLinodeFirewallCreate,
		Read:   resourceLinodeFirewallRead,
		Update: resourceLinodeFirewallUpdate,
		Delete: resourceLinodeFirewallDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"label": {
				Type:         schema.TypeString,
				Description:  "The label for the Firewall. For display purposes only. If no label is provided, a default will be assigned.",
				Required:     true,
				ValidateFunc: validation.StringLenBetween(3, 32),
			},
			"tags": {
				Type:        schema.TypeSet,
				Description: "An array of tags applied to this object. Tags are for organizational purposes only.",
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
				Set:         schema.HashString,
			},
			"disabled": {
				Type:        schema.TypeBool,
				Description: "If true, the Firewall is inactive.",
				Optional:    true,
				Default:     false,
			},
			"inbound": {
				Type:        schema.TypeList,
				Elem:        resourceLinodeFirewallRule(),
				Description: "A firewall rule that specifies what inbound network traffic is allowed.",
				Optional:    true,
			},
			"inbound_policy": {
				Type:        schema.TypeString,
				Description: "The default behavior for inbound traffic. This setting can be overridden by updating the inbound.action property for an individual Firewall Rule.",
				Required:    true,
			},
			"outbound": {
				Type:        schema.TypeList,
				Elem:        resourceLinodeFirewallRule(),
				Description: "A firewall rule that specifies what outbound network traffic is allowed.",
				Optional:    true,
			},
			"outbound_policy": {
				Type:        schema.TypeString,
				Description: "The default behavior for outbound traffic. This setting can be overridden by updating the outbound.action property for an individual Firewall Rule.",
				Required:    true,
			},
			"linodes": {
				Type:        schema.TypeSet,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "The IDs of Linodes to apply this firewall to.",
				Optional:    true,
				Set:         schema.HashInt,
			},
			"devices": {
				Type: schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Description: "The ID of the firewall device.",
							Computed:    true,
						},
						"entity_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The ID of the underlying entity for the firewall device (e.g. the Linode's ID).",
						},
						"type": {
							Type:        schema.TypeString,
							Description: "The type of firewall device.",
							Computed:    true,
						},
						"label": {
							Type:        schema.TypeString,
							Description: "The label of the underlying entity for the firewall device.",
							Computed:    true,
						},
						"url": {
							Type:        schema.TypeString,
							Description: "The URL of the underlying entity for the firewall device.",
							Computed:    true,
						},
					},
				},
				Computed:    true,
				Description: "The devices associated with this firewall.",
			},
			"status": {
				Type:        schema.TypeString,
				Description: "The status of the firewall.",
				Computed:    true,
			},
		},
	}
}

func resourceLinodeFirewallRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ProviderMeta).Client
	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return fmt.Errorf("failed to parse Firewall %s as int: %s", d.Id(), err)
	}

	firewall, err := client.GetFirewall(context.Background(), id)
	if err != nil {
		return fmt.Errorf("failed to get firewall %d: %s", id, err)
	}

	rules, err := client.GetFirewallRules(context.Background(), id)
	if err != nil {
		return fmt.Errorf("failed to get rules for firewall %d: %s", id, err)
	}

	devices, err := client.ListFirewallDevices(context.Background(), id, nil)
	if err != nil {
		return fmt.Errorf("failed to get devices for firewall %d: %s", id, err)
	}

	d.Set("label", firewall.Label)
	d.Set("disabled", firewall.Status == linodego.FirewallDisabled)
	d.Set("tags", firewall.Tags)
	d.Set("status", firewall.Status)
	d.Set("inbound", flattenLinodeFirewallRules(rules.Inbound))
	d.Set("outbound", flattenLinodeFirewallRules(rules.Outbound))
	d.Set("inbound_policy", firewall.Rules.InboundPolicy)
	d.Set("outbound_policy", firewall.Rules.OutboundPolicy)
	d.Set("linodes", flattenLinodeFirewallLinodes(devices))
	d.Set("devices", flattenLinodeFirewallDevices(devices))
	return nil
}

func resourceLinodeFirewallCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ProviderMeta).Client

	createOpts := linodego.FirewallCreateOptions{
		Label: d.Get("label").(string),
		Tags:  expandStringSet(d.Get("tags").(*schema.Set)),
	}
	createOpts.Devices.Linodes = expandIntSet(d.Get("linodes").(*schema.Set))
	createOpts.Rules.Inbound = expandLinodeFirewallRules(d.Get("inbound").([]interface{}))
	createOpts.Rules.InboundPolicy = d.Get("inbound_policy").(string)
	createOpts.Rules.Outbound = expandLinodeFirewallRules(d.Get("outbound").([]interface{}))
	createOpts.Rules.OutboundPolicy = d.Get("outbound_policy").(string)

	if len(createOpts.Rules.Inbound)+len(createOpts.Rules.Outbound) == 0 {
		return errors.New("cannot create firewall without at least one inbound or outbound rule")
	}

	firewall, err := client.CreateFirewall(context.Background(), createOpts)
	if err != nil {
		return fmt.Errorf("failed to create Firewall: %s", err)
	}
	d.SetId(strconv.Itoa(firewall.ID))

	if d.Get("disabled").(bool) {
		if _, err := client.UpdateFirewall(context.Background(), firewall.ID, linodego.FirewallUpdateOptions{
			Status: linodego.FirewallDisabled,
		}); err != nil {
			return fmt.Errorf("failed to disable firewall %d: %s", firewall.ID, err)
		}
	}

	return resourceLinodeFirewallRead(d, meta)
}

func resourceLinodeFirewallUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ProviderMeta).Client
	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return fmt.Errorf("failed to parse Firewall %s as int: %s", d.Id(), err)
	}

	if d.HasChanges("label", "tags", "disabled") {
		updateOpts := linodego.FirewallUpdateOptions{}
		if d.HasChange("label") {
			updateOpts.Label = d.Get("label").(string)
		}
		if d.HasChange("tags") {
			tags := expandStringSet(d.Get("tags").(*schema.Set))
			updateOpts.Tags = &tags
		}
		if d.HasChange("disabled") {
			updateOpts.Status = expandLinodeFirewallStatus(d.Get("disabled"))
		}

		if _, err := client.UpdateFirewall(context.Background(), id, updateOpts); err != nil {
			return fmt.Errorf("failed to update firewall %d: %s", id, err)
		}
	}

	inboundRules := expandLinodeFirewallRules(d.Get("inbound").([]interface{}))
	outboundRules := expandLinodeFirewallRules(d.Get("outbound").([]interface{}))
	ruleSet := linodego.FirewallRuleSet{
		Inbound:        inboundRules,
		InboundPolicy:  d.Get("inbound_policy").(string),
		Outbound:       outboundRules,
		OutboundPolicy: d.Get("outbound_policy").(string),
	}
	if _, err := client.UpdateFirewallRules(context.Background(), id, ruleSet); err != nil {
		return fmt.Errorf("failed to update rules for firewall %d: %s", id, err)
	}

	linodes := expandIntSet(d.Get("linodes").(*schema.Set))
	devices, err := client.ListFirewallDevices(context.Background(), id, nil)
	if err != nil {
		return fmt.Errorf("failed to get devices for firewall %d: %s", id, err)
	}

	provisionedLinodes := make(map[int]linodego.FirewallDevice)
	for _, device := range devices {
		if device.Entity.Type == linodego.FirewallDeviceLinode {
			provisionedLinodes[device.Entity.ID] = device
		}
	}

	// keep track of all visited linodes for accounting
	visitedLinodes := make(map[int]struct{})

	for _, linodeID := range linodes {
		if _, ok := provisionedLinodes[linodeID]; !ok {
			if _, err := client.CreateFirewallDevice(context.Background(), id, linodego.FirewallDeviceCreateOptions{
				ID:   linodeID,
				Type: linodego.FirewallDeviceLinode,
			}); err != nil {
				return fmt.Errorf("failed to create firewall device for linode %d: %s", linodeID, err)
			}
		}

		visitedLinodes[linodeID] = struct{}{}
	}

	// ensure there are no provisioned firewall devices for which there is no
	// declared reference.
	for linodeID, device := range provisionedLinodes {
		if _, ok := visitedLinodes[linodeID]; !ok {
			if err := client.DeleteFirewallDevice(context.Background(), id, device.ID); err != nil {
				return fmt.Errorf("failed to delete firewall device %d: %s", id, err)
			}
		}
	}

	return nil
}

func resourceLinodeFirewallDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ProviderMeta).Client
	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return fmt.Errorf("failed to parse Firewall %s as int: %s", d.Id(), err)
	}

	if err := client.DeleteFirewall(context.Background(), id); err != nil {
		return fmt.Errorf("failed to delete Firewall %d: %s", id, err)
	}
	return nil
}

func expandLinodeFirewallRules(ruleSpecs []interface{}) []linodego.FirewallRule {
	rules := make([]linodego.FirewallRule, len(ruleSpecs))
	for i, ruleSpec := range ruleSpecs {
		ruleSpec := ruleSpec.(map[string]interface{})
		rule := linodego.FirewallRule{}

		rule.Label = ruleSpec["label"].(string)
		rule.Action = ruleSpec["action"].(string)
		rule.Protocol = linodego.NetworkProtocol(ruleSpec["protocol"].(string))
		rule.Ports = ruleSpec["ports"].(string)

		ipv4 := expandStringList(ruleSpec["ipv4"].([]interface{}))
		if len(ipv4) > 0 {
			rule.Addresses.IPv4 = &ipv4
		}
		ipv6 := expandStringList(ruleSpec["ipv6"].([]interface{}))
		if len(ipv6) > 0 {
			rule.Addresses.IPv6 = &ipv6
		}
		rules[i] = rule
	}
	return rules
}

func flattenLinodeFirewallRules(rules []linodego.FirewallRule) []map[string]interface{} {
	specs := make([]map[string]interface{}, len(rules))
	for i, rule := range rules {
		specs[i] = map[string]interface{}{
			"label":    rule.Label,
			"action":   rule.Action,
			"protocol": rule.Protocol,
			"ports":    rule.Ports,
			"ipv4":     rule.Addresses.IPv4,
			"ipv6":     rule.Addresses.IPv6,
		}
	}
	return specs
}

func flattenLinodeFirewallLinodes(devices []linodego.FirewallDevice) []int {
	linodes := make([]int, 0, len(devices))
	for _, device := range devices {
		if device.Entity.Type == linodego.FirewallDeviceLinode {
			linodes = append(linodes, device.Entity.ID)
		}
	}
	return linodes
}

func flattenLinodeFirewallDevices(devices []linodego.FirewallDevice) []map[string]interface{} {
	governedDevices := make([]map[string]interface{}, len(devices))
	for i, device := range devices {
		governedDevices[i] = map[string]interface{}{
			"id":        device.ID,
			"entity_id": device.Entity.ID,
			"type":      device.Entity.Type,
			"label":     device.Entity.Label,
			"url":       device.Entity.URL,
		}
	}
	return governedDevices
}

func expandLinodeFirewallStatus(disabled interface{}) linodego.FirewallStatus {
	return map[bool]linodego.FirewallStatus{
		true:  linodego.FirewallDisabled,
		false: linodego.FirewallEnabled,
	}[disabled.(bool)]
}
