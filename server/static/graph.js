var graph = new Rickshaw.Graph( {
  element: document.getElementById("chart"),
  width: 960,
  height: 500,
  renderer: 'line',
  interpolation: 'linear',
  series: [
    {
      color: "#c05020",
      data: params.DataPoints,
      name: params.ResourceID + "." + params.MetricKey,
    },
  ]
} );

graph.render();

var hoverDetail = new Rickshaw.Graph.HoverDetail( {
  graph: graph
} );

var legend = new Rickshaw.Graph.Legend( {
  graph: graph,
  element: document.getElementById('legend')

} );

var shelving = new Rickshaw.Graph.Behavior.Series.Toggle( {
  graph: graph,
  legend: legend
} );

var axes = new Rickshaw.Graph.Axis.Time( {
  graph: graph
} );
axes.render();
