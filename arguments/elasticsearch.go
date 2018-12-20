package arguments

import (
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"net/url"
	"strings"
)

type ElasticsearchArgs struct {
	Nodes []string
	IndexName string
}

func (args *ElasticsearchArgs) Verify() error {
	v := verifier.New()
	for _, node := range args.Nodes {
		_, err := url.Parse(node)
		v.That(err == nil, "invalid node: %s", node)
	}
	v.That(args.IndexName != "", "Elasticsearch index name is empty")
	return v.GetError()
}

func (args *ElasticsearchArgs) Populate(c *cli.Context) {
	args.Nodes = c.GlobalStringSlice("elasticsearch-url")
	if len(args.Nodes) == 0 {
		args.Nodes = []string{"http://127.0.0.1:9200"}
	}
	args.IndexName = strings.TrimSpace(c.GlobalString("elasticsearch-index-name"))
}
