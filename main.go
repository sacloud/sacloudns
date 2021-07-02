package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
	"github.com/miekg/dns"
	"github.com/sacloud/libsacloud/v2/sacloud"
	"github.com/sacloud/libsacloud/v2/sacloud/search"
	"github.com/sacloud/libsacloud/v2/sacloud/types"
)

// version by Makefile
var version string

type verOpts struct {
}

type listOpts struct {
}

type zoneOpts struct {
	Name string `long:"name" description:"dnszone name to find"`
}

type fzoneOpts struct {
	Name string `long:"name" description:"record name to find zone"`
}

type raddOpts struct {
	Zone        string        `long:"zone" description:"dnszone name to add a record"`
	TTL         int           `long:"ttl" description:"record TTL to add" default:"300"`
	Name        string        `long:"name" description:"record NAME or FQDN(with final dot) to add" required:"true"`
	Type        string        `long:"type" description:"record TYPE to add" required:"true"`
	RData       string        `long:"data" description:"record DATA to add" required:"true"`
	Wait        bool          `long:"wait" description:"wait for record propagation"`
	WaitTimeout time.Duration `long:"wait-timeout" description:"wait timeout for record propagation" default:"60s"`
}

type rsetOpts struct {
	Zone        string        `long:"zone" description:"dnszone name to set a record"`
	TTL         int           `long:"ttl" description:"record TTL to set" default:"300"`
	Name        string        `long:"name" description:"record NAME or FQDN(with final dot) to set" required:"true"`
	Type        string        `long:"type" description:"record TYPE to set" required:"true"`
	RData       string        `long:"data" description:"record DATA to set" required:"true"`
	Wait        bool          `long:"wait" description:"wait for record propagation"`
	WaitTimeout time.Duration `long:"wait-timeout" description:"wait timeout for record propagation" default:"60s"`
}

type rdelOpts struct {
	Zone  string `long:"zone" description:"dnszone name to delete a record"`
	Name  string `long:"name" description:"record NAME or FQDN(with final dot) to delete" required:"true"`
	Type  string `long:"type" description:"record TYPE to delete" required:"true"`
	RData string `long:"data" description:"record DATA to delete" required:"true"`
}

type mainOpts struct {
	ListCmd    listOpts  `command:"list" description:"list zones"`
	ZoneCmd    zoneOpts  `command:"zone" description:"describe zone"`
	FzoneCmd   fzoneOpts `command:"fzone" description:"find zone for the record"`
	RAddCmd    raddOpts  `command:"radd" description:"add a record"`
	RSetCmd    rsetOpts  `command:"rset" description:"replace records or add a record"`
	RDelCmd    rdelOpts  `command:"rdelete" description:"delete a record"`
	VersionCMD verOpts   `command:"version" description:"display version"`
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

func waitPropagation(timeout, interval time.Duration, zone *sacloud.DNS, r *sacloud.DNSRecord) error {
	var lastErr error
	timeUp := time.After(timeout)
	log.Printf("Checking DNS record propagation.")
	for {
		select {
		case <-timeUp:
			return fmt.Errorf("timeout: last error: %w", lastErr)
		default:
		}

		stop, err := checkPropagation(zone, r)
		if stop {
			return err
		}
		if err != nil {
			lastErr = err
		}
		log.Printf("Waiting for DNS record propagation.")
		time.Sleep(interval)
	}

}

func availPropagation(t string) error {
	switch strings.ToUpper(t) {
	case "TXT":
		return nil
	case "CNAME":
		return nil
	default:
		return fmt.Errorf("--wait isnt available for type '%s'", t)
	}
}

func checkPropagation(zone *sacloud.DNS, r *sacloud.DNSRecord) (bool, error) {
	result, err := dnsQuery(zone, r)
	if err != nil {
		return false, err
	}
	if result.Rcode != dns.RcodeSuccess {
		return false, fmt.Errorf("error %s", dns.RcodeToString[result.Rcode])
	}

	var found = false
	for _, ans := range result.Answer {
		switch r.Type {
		case types.DNSRecordTypes.TXT:
			if a, ok := ans.(*dns.TXT); ok {
				d := strings.Join(a.Txt, "")
				if d == r.RData {
					found = true
					break
				}
			}
		case types.DNSRecordTypes.CNAME:
			if a, ok := ans.(*dns.CNAME); ok {
				if a.Target == r.RData {
					found = true
					break
				}
			}
		}
	}
	return found, nil
}

func dnsQuery(zone *sacloud.DNS, r *sacloud.DNSRecord) (*dns.Msg, error) {
	rtype, ok := dns.StringToType[r.Type.String()]
	if !ok {
		return nil, fmt.Errorf("invalid type: %s", r.Type.String())
	}
	fqdn := zone.DNSZone
	if r.Name != "@" {
		fqdn = r.Name + "." + fqdn + "."
	}

	m := new(dns.Msg)
	m.SetQuestion(fqdn, rtype)
	m.SetEdns0(4096, false)
	m.RecursionDesired = false

	var in *dns.Msg
	var err error

	for _, ns := range zone.DNSNameServers {
		in, err = sendDNSQuery(m, ns+":53")
		if err == nil && len(in.Answer) > 0 {
			break
		}
	}
	return in, err
}

var dnsTimeout = 5 * time.Second

func sendDNSQuery(m *dns.Msg, ns string) (*dns.Msg, error) {
	udp := &dns.Client{Net: "udp", Timeout: dnsTimeout}
	in, _, err := udp.Exchange(m, ns)

	if in != nil && in.Truncated {
		tcp := &dns.Client{Net: "tcp", Timeout: dnsTimeout}
		in, _, err = tcp.Exchange(m, ns)
	}

	return in, err
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
	searchName := name
	if len(searchName) > 63 {
		searchName = searchName[0:63]
	}
	condition.Filter[search.Key("Name")] = search.PartialMatch(searchName)
	result, err := searchZone(context.Background(), condition)
	if err != nil {
		return nil, err
	}
	for _, d := range result.DNS {
		if d.DNSZone == name {
			return d, nil
		}
	}
	// If zone is not found in result by Name, get all zones.
	result, err = searchZone(context.Background(), &sacloud.FindCondition{})
	if err != nil {
		return nil, err
	}
	for _, d := range result.DNS {
		if d.DNSZone == name {
			return d, nil
		}
	}
	return nil, fmt.Errorf("not found: zone '%s'", name)
}

func rewriteName(Name, Zone string) string {
	if !strings.HasSuffix(Name, ".") {
		return Name
	}
	return strings.TrimSuffix(Name, "."+Zone+".")
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

func searchZoneForRecord(ctx context.Context, record string) (*sacloud.DNS, error) {
	testRecord := record
	result, err := searchZone(ctx, &sacloud.FindCondition{})
	if err != nil {
		return nil, err
	}
	for testRecord != "" {
		for _, z := range result.DNS {
			if z.DNSZone == testRecord {
				return z, nil
			}
		}
		if strings.Contains(testRecord, ".") {
			t := strings.SplitN(testRecord, ".", 2)
			testRecord = t[1]
			continue
		}
		break
	}
	return nil, fmt.Errorf("Could not find zone for %s", record)
}

func (opts *fzoneOpts) Execute(args []string) error {
	if opts.Name == "" && len(args) > 0 {
		opts.Name = args[0]
	}
	result, err := searchZoneForRecord(context.Background(), opts.Name)
	if err != nil {
		return err
	}
	os.Stdout.Write([]byte(result.DNSZone + "\n"))
	return nil
}

func (opts *listOpts) Execute(args []string) error {
	result, err := searchZone(context.Background(), &sacloud.FindCondition{})
	if err != nil {
		return err
	}
	return outJSON(result)
}

func (opts *raddOpts) Execute(args []string) error {
	if opts.Wait {
		if err := availPropagation(opts.Type); err != nil {
			return err
		}
	}
	zone, err := fetchZone(context.Background(), opts.Zone)
	if err != nil {
		return err
	}
	opts.Name = rewriteName(opts.Name, opts.Zone)
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
	if opts.Wait && new.Type == types.DNSRecordTypes.TXT {
		err = waitPropagation(opts.WaitTimeout, 2*time.Second, zone, new)
		if err != nil {
			return err
		}
	}

	return outJSON(result)
}

func (opts *rsetOpts) Execute(args []string) error {
	if opts.Wait {
		if err := availPropagation(opts.Type); err != nil {
			return err
		}
	}
	zone, err := fetchZone(context.Background(), opts.Zone)
	if err != nil {
		return err
	}
	opts.Name = rewriteName(opts.Name, opts.Zone)
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
	if opts.Wait && new.Type == types.DNSRecordTypes.TXT {
		err = waitPropagation(opts.WaitTimeout, 2*time.Second, zone, new)
		if err != nil {
			return err
		}
	}
	return outJSON(result)
}

func (opts *rdelOpts) Execute(args []string) error {
	zone, err := fetchZone(context.Background(), opts.Zone)
	if err != nil {
		return err
	}
	opts.Name = rewriteName(opts.Name, opts.Zone)
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

func (opts *verOpts) Execute(args []string) error {
	fmt.Printf(`%s %s
Compiler: %s %s
`,
		os.Args[0],
		version,
		runtime.Compiler,
		runtime.Version())
	return nil
}

func main() {
	godotenv.Load()
	opts := mainOpts{}
	psr := flags.NewParser(&opts, flags.HelpFlag)
	_, err := psr.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
