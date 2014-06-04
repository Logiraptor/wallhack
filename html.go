package main

var html = `
<!DOCTYPE html>
<html>
<title>Wallhack</title>

<xmp theme="cerulean" style="display:none;">

{{range $routerName, $router := .}}


# {{$routerName}}

### Documentation

{{$router.Doc | trust}}

### Contents

Name | URL
-----|-----{{range $router.Endpoints}}
<a href="#{{.Func}}">{{.Func}}</a> | {{.Method}} {{.URL}}{{end}}

{{range $router.Endpoints}}

<a id="{{.Func}}" href="#top">top</a>
#### {{.Func}}

    {{.Method}} {{.URL}}

{{.Doc | trust}}

{{if .Response}}
Response:

    {{.Response | json}}{{end}}

{{end}}
{{end}}

</xmp>

<script src="http://strapdownjs.com/v/0.2/strapdown.js"></script>
</html>
`
