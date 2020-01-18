#!/usr/bin/env python
import pydot
import sys
from typing import Dict, List


def usage():
    usage_text = """Usage: {0} "$(cat graph1)" "$(cat graph2)"

This tool takes two graphs and finds all the nodes in the second graph that
don't exist in the first graph. All these nodes are then colored in grey and
the final graph is printed back.

Example: {0} \\
    "$(git show HEAD^:pkg/defkinds/webserver/testdata/build/from-git-context.json | zbuild llbgraph)" \\
    "$(cat pkg/defkinds/webserver/testdata/build/from-git-context.json | zbuild llbgraph)" \\
    | dot /dev/stdin -o /dev/stdout -T png | feh - """
    
    print(usage_text.format(sys.argv[0]), file=sys.stderr)


def list_edges(graph: pydot.Graph) -> Dict[str, str]:
    # Source node is stored as key, dest node as value
    edges = {}

    for edge in graph.get_edge_list():
        edges[edge.get_source()] = edge.get_destination()
    
    return edges


def find_children_nodes(graph: pydot.Graph, traversed: List[str], node_name: str) -> List[str]:
    children = []

    for edge in graph.get_edge(node_name):
        dest = edge.get_destination()
        if dest not in traversed:
            children.append(dest)
    
    return children


def color_nodes(graph: pydot.Graph, nodes: List[str]):
    for node_name in nodes:
        node = graph.get_node(node_name)[0]
        node.set_fillcolor("#5f5f5f5f")
        node.set_style("filled")


def diff_graphs(origin_graph: pydot.Graph, updated_graph: pydot.Graph):
    origin_edges = list_edges(origin_graph)
    traversed = []

    for node in updated_graph.get_nodes():
        if node in traversed:
            continue

        node_name = node.get_name()
        children = find_children_nodes(updated_graph, traversed, node_name)
        to_color = [node_name] + children
        traversed += to_color

        if node_name in origin_edges:
            continue

        color_nodes(updated_graph, to_color)
    
    print(updated_graph.to_string())


if __name__ == "__main__":
    if len(sys.argv) != 3:
        usage()
        sys.exit(1)

    origin = pydot.graph_from_dot_data(sys.argv[1])[0]
    updated = pydot.graph_from_dot_data(sys.argv[2])[0]

    diff_graphs(origin, updated)
