package resolver

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/miekg/dns"
)

// HttpsResolver is a resolver using just a single tcp connection with pipelining.
type HttpsResolver struct {
	BasicResolverConn
	Client *http.Client
}

// HttpsQuery holds the query information for a httpsResolverConn.
type HttpsQuery struct {
	Query    *Query
	Response chan *dns.Msg
}

// MakeCacheRecord creates an RRCache record from a reply.
func (tq *HttpsQuery) MakeCacheRecord(reply *dns.Msg, resolverInfo *ResolverInfo) *RRCache {
	return &RRCache{
		Domain:   tq.Query.FQDN,
		Question: tq.Query.QType,
		RCode:    reply.Rcode,
		Answer:   reply.Answer,
		Ns:       reply.Ns,
		Extra:    reply.Extra,
		Resolver: resolverInfo.Copy(),
	}
}

// NewHTTPSResolver returns a new HttpsResolver.
func NewHTTPSResolver(resolver *Resolver) *HttpsResolver {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: resolver.VerifyDomain,
			// TODO: use portbase rng
		},
	}

	client := &http.Client{Transport: tr}

	newResolver := &HttpsResolver{
		BasicResolverConn: BasicResolverConn{
			resolver: resolver,
		},
		Client: client,
	}
	newResolver.BasicResolverConn.init()
	return newResolver
}

// Query executes the given query against the resolver.
func (hr *HttpsResolver) Query(ctx context.Context, q *Query) (*RRCache, error) {
	// Get resolver connection.
	dnsQuery := new(dns.Msg)
	dnsQuery.SetQuestion(q.FQDN, uint16(q.QType))

	buf, err := dnsQuery.Pack()

	if err != nil {
		return nil, err
	}
	b64dns := base64.RawStdEncoding.EncodeToString(buf)

	url := &url.URL{
		Scheme:     "https",
		Host:       hr.resolver.ServerAddress,
		Path:       hr.resolver.Path,
		ForceQuery: true,
		RawQuery:   fmt.Sprintf("dns=%s", b64dns),
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)

	if err != nil {
		return nil, err
	}

	resp, err := hr.Client.Do(request)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}
	reply := new(dns.Msg)
	err = reply.Unpack(body)

	if err != nil {
		return nil, err
	}

	newRecord := &RRCache{
		Domain:   q.FQDN,
		Question: q.QType,
		RCode:    reply.Rcode,
		Answer:   reply.Answer,
		Ns:       reply.Ns,
		Extra:    reply.Extra,
		Resolver: hr.resolver.Info.Copy(),
	}

	// TODO: check if reply.Answer is valid
	return newRecord, nil
}
