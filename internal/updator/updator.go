package updator

import (
	"context"
	"net"

	"github.com/favonia/cloudflare-ddns-go/internal/api"
	"github.com/favonia/cloudflare-ddns-go/internal/ipnet"
	"github.com/favonia/cloudflare-ddns-go/internal/pp"
	"github.com/favonia/cloudflare-ddns-go/internal/quiet"
)

// Args is the type of (named) arguments to updateRecords.
type Args struct {
	Handle    api.Handle
	IPNetwork ipnet.Type
	IP        net.IP
	Domain    api.FQDN
	TTL       api.TTL
	Proxied   bool
}

func SplitRecords(rmap map[string]net.IP, target net.IP) (matchedIDs, unmatchedIDs []string) {
	for id, ip := range rmap {
		if ip.Equal(target) {
			matchedIDs = append(matchedIDs, id)
		} else {
			unmatchedIDs = append(unmatchedIDs, id)
		}
	}

	return matchedIDs, unmatchedIDs
}

func Do(ctx context.Context, indent pp.Indent, quiet quiet.Quiet, args *Args) bool { //nolint:funlen,cyclop,gocognit
	recordType := args.IPNetwork.RecordType()

	rs, ok := args.Handle.ListRecords(ctx, indent, args.Domain, args.IPNetwork)
	if !ok {
		pp.Printf(indent, pp.EmojiError, "Failed to update %s records of %s.", recordType, args.Domain)
		return false
	}

	matchedIDs, unmatchedIDs := SplitRecords(rs, args.IP)

	// whether there was already an up-to-date record
	uptodate := false
	// whether everything works
	numUnmatched := len(unmatchedIDs)

	// delete every record if ip is `nil`
	if args.IP == nil {
		uptodate = true
	}

	if !uptodate && len(matchedIDs) > 0 {
		uptodate = true
		matchedIDs = matchedIDs[1:]
	}

	if uptodate && len(matchedIDs) == 0 && len(unmatchedIDs) == 0 {
		if !quiet {
			pp.Printf(indent, pp.EmojiAlreadyDone, "The %s records of %s are already up to date.", recordType, args.Domain)
		}

		return true
	}

	if !uptodate && args.IP != nil {
		var unhandled []string

		for i, id := range unmatchedIDs {
			if args.Handle.UpdateRecord(ctx, indent, args.Domain, args.IPNetwork, id, args.IP) {
				pp.Printf(indent, pp.EmojiUpdateRecord,
					"Updated a stale %s record of %s (ID: %s).", recordType, args.Domain, id)

				uptodate = true
				numUnmatched--
				unhandled = unmatchedIDs[i+1:]

				break
			} else {
				if args.Handle.DeleteRecord(ctx, indent, args.Domain, args.IPNetwork, id) {
					pp.Printf(indent, pp.EmojiDelRecord,
						"Deleted a stale %s record of %s instead (ID: %s).", recordType, args.Domain, id)
					numUnmatched--
				}
				continue
			}
		}

		unmatchedIDs = unhandled
	}

	if !uptodate && args.IP != nil {
		if id, ok := args.Handle.CreateRecord(ctx, indent,
			args.Domain, args.IPNetwork, args.IP, args.TTL.Int(), args.Proxied); ok {
			pp.Printf(indent, pp.EmojiAddRecord,
				"Added a new %s record of %s (ID: %s).", recordType, args.Domain, id)
			uptodate = true
		}
	}

	for _, id := range unmatchedIDs {
		if args.Handle.DeleteRecord(ctx, indent, args.Domain, args.IPNetwork, id) {
			pp.Printf(indent, pp.EmojiDelRecord,
				"Deleted a stale %s record of %s (ID: %s).", recordType, args.Domain, id)
			numUnmatched--
		}
	}

	for _, id := range matchedIDs {
		if args.Handle.DeleteRecord(ctx, indent, args.Domain, args.IPNetwork, id) {
			pp.Printf(indent, pp.EmojiDelRecord,
				"Deleted a duplicate %s record of %s (ID: %s).", recordType, args.Domain, id)
		}
	}

	if !uptodate || numUnmatched > 0 {
		pp.Printf(indent, pp.EmojiError,
			"Failed to update %s records of %s.", recordType, args.Domain)
		return false
	}

	return true
}
