package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
	"github.com/sacloud/libsacloud/v2/sacloud"
	"github.com/sacloud/libsacloud/v2/sacloud/search"
	"github.com/sacloud/libsacloud/v2/sacloud/types"
)

type listOpts struct {
}

type zoneOpts struct {
	Name string `long:"name" description:"dnszone name to find"`
}

type raddOpts struct {
	Zone  string `long:"zone" description:"dnszone name to add a record"`
	TTL   int    `long:"ttl" description:"record TTL to add" default:"300"`
	Name  string `long:"name" description:"record NAME to add" required:"true"`
	Type  string `long:"type" description:"record TYPE to add" required:"true"`
	RData string `long:"data" description:"record DATA to add" required:"true"`
}

type rsetOpts struct {
	Zone  string `long:"zone" description:"dnszone name to set a record"`
	TTL   int    `long:"ttl" description:"record TTL to set" default:"300"`
	Name  string `long:"name" description:"record NAME to set" required:"true"`
	Type  string `long:"type" description:"record TYPE to set" required:"true"`
	RData string `long:"data" description:"record DATA to set" required:"true"`
}

type rdelOpts struct {
	Zone  string `long:"zone" description:"dnszone name to set a record"`
	Name  string `long:"name" description:"record NAME to set" required:"true"`
	Type  string `long:"type" description:"record TYPE to set" required:"true"`
	RData string `long:"data" description:"record DATA to set" required:"true"`
}

type mainOpts struct {
	ListCmd listOpts `command:"list" description:"list zones"`
	ZoneCmd zoneOpts `command:"zone" description:"describe zone"`
	RAddCmd raddOpts `command:"radd" description:"add a record"`
	RSetCmd rsetOpts `command:"rset" description:"replace records or add a record"`
	RDelCmd rdelOpts `command:"rdelete" description:"delete a record"`
}

func outJSON(result interface{}) error {
	json, err := json.Marshal(result)
	if err != nil {
		return err
	}
	os.Stdout.Write(json)
	return nil
}

func dnsClient() (sacloud.DNSAPI, error) {
	client, err := sacloud.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return sacloud.NewDNSOp(client), nil
}

func searchZone(ctx context.Context, condition *sacloud.FindCondition) (*sacloud.DNSFindResult, error) {
	dnsOp, err := dnsClient()
	if err != nil {
		return nil, err
	}
	return dnsOp.Find(ctx, condition)
}

func fetchZone(ctx context.Context, name string) (*sacloud.DNS, error) {
	condition := &sacloud.FindCondition{
		Filter: map[search.FilterKey]interface{}{},
	}
	condition.Filter[search.Key("Name")] = search.ExactMatch(name)
	result, err := searchZone(context.Background(), condition)
	if err != nil {
		return nil, err
	}
	if result.Count == 0 {
		return nil, fmt.Errorf("not found: zone '%s'", name)
	}
	return result.DNS[0], nil
}

func (opts *zoneOpts) Execute(args []string) error {
	if opts.Name == "" && len(args) > 0 {
		opts.Name = args[0]
	}
	zone, err := fetchZone(context.Background(), opts.Name)
	if err != nil {
		return err
	}
	return outJSON(zone)
}

func (opts *listOpts) Execute(args []string) error {
	result, err := searchZone(context.Background(), &sacloud.FindCondition{})
	if err != nil {
		return err
	}
	return outJSON(result)
}

func (opts *raddOpts) Execute(args []string) error {
	zone, err := fetchZone(context.Background(), opts.Zone)
	if err != nil {
		return err
	}
	new := sacloud.NewDNSRecord(
		types.EDNSRecordType(opts.Type),
		opts.Name,
		opts.RData,
		opts.TTL,
	)
	err = new.Validate()
	if err != nil {
		return err
	}
	records := zone.GetRecords()
	now := records.Find(opts.Name, types.EDNSRecordType(opts.Type), opts.RData)
	if now != nil {
		return fmt.Errorf("record %v exists for zone %s", now, opts.Zone)
	}
	records.Add(new)
	updateReq := &sacloud.DNSUpdateRequest{
		Records: records,
	}

	dnsOp, _ := dnsClient()
	result, err := dnsOp.Update(context.Background(), zone.ID, updateReq)
	if err != nil {
		return err
	}
	return outJSON(result)
}

func (opts *rsetOpts) Execute(args []string) error {
	zone, err := fetchZone(context.Background(), opts.Zone)
	if err != nil {
		return err
	}
	new := sacloud.NewDNSRecord(
		types.EDNSRecordType(opts.Type),
		opts.Name,
		opts.RData,
		opts.TTL,
	)
	err = new.Validate()
	if err != nil {
		return err
	}
	records := zone.GetRecords()
	newRecords := make([]*sacloud.DNSRecord, 0)
	for _, r := range records {
		if r.Name == new.Name && r.Type == new.Type {
			continue
		}
		newRecords = append(newRecords, r)
	}
	newRecords = append(newRecords, new)
	updateReq := &sacloud.DNSUpdateRequest{
		Records: newRecords,
	}
	dnsOp, _ := dnsClient()
	result, err := dnsOp.Update(context.Background(), zone.ID, updateReq)
	if err != nil {
		return err
	}
	return outJSON(result)
}

func (opts *rdelOpts) Execute(args []string) error {
	zone, err := fetchZone(context.Background(), opts.Zone)
	if err != nil {
		return err
	}
	delete := sacloud.NewDNSRecord(
		types.EDNSRecordType(opts.Type),
		opts.Name,
		opts.RData,
		42,
	)
	err = delete.Validate()
	if err != nil {
		return err
	}
	records := zone.GetRecords()
	records.Delete(delete)
	updateReq := &sacloud.DNSUpdateRequest{
		Records: records,
	}

	dnsOp, _ := dnsClient()
	result, err := dnsOp.Update(context.Background(), zone.ID, updateReq)
	if err != nil {
		return err
	}
	return outJSON(result)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	opts := mainOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err = psr.Parse()
	if err != nil {
		os.Exit(1)
	}
}
