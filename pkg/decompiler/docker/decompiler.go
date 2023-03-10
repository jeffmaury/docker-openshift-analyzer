package decompiler

import (
	"context"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	decompilerutils "github.com/redhat-developer/docker-openshift-analyzer/pkg/decompiler/utils"
	"github.com/redhat-developer/docker-openshift-analyzer/pkg/utils"
	"regexp"
	"sort"
	"strings"
)

type OrderedHistory []image.HistoryResponseItem

func (o OrderedHistory) Len() int {
	return len(o)
}

func (o OrderedHistory) Less(i, j int) bool {
	return o[i].Created < o[j].Created
}

func (o OrderedHistory) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

type DockerProvider struct{}

func (p DockerProvider) Decompile(imageName string) (*parser.Node, error) {
	ctx, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, nil
	}
	history, err := ctx.ImageHistory(context.Background(), imageName)
	if err != nil {
		return nil, nil
	}

	root := &parser.Node{}
	sort.Sort(OrderedHistory(history))
	for _, hist := range history {
		if hist.Comment != "" && strings.HasPrefix(strings.ToUpper(hist.Comment), utils.FROM_INSTRUCTION) &&
			hist.Size != 0 {
			err := decompilerutils.Line2Node(hist.Comment, root)
			if err != nil {
				return nil, err
			}
		}
		if hist.CreatedBy != "" {
			cmd := decompilerutils.ExtractCmd(hist.CreatedBy)
			if cmd != "" {
				err := decompilerutils.Line2Node(cmd, root)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	parseTree(root)
	return root, nil
}

var portExpr, _ = regexp.Compile("(?:map\\[)?(\\d+\\/(?:tcp|udp))\\:{}\\]?")

func parseTree(node *parser.Node) {
	for _, child := range node.Children {
		if child.Value+" " == utils.EXPOSE_INSTRUCTION {
			next := child.Next
			for next != nil {
				ports := portExpr.FindStringSubmatch(next.Value)
				if ports != nil {
					next.Value = ports[1]
				}
				next = next.Next
			}
		}
	}
}
