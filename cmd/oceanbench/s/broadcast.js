function init() {
    document.getElementById('time-zone').value = getTimezone();
    {{if .CurrentBroadcast.StartTimeUnix}}sync('start-time', 'start-time-unix', 'time-zone', false);{{end}}
    {{if .CurrentBroadcast.EndTimeUnix}}sync('end-time', 'end-time-unix', 'time-zone', false);{{end}}

    {{ range .CurrentBroadcast.SensorList }}
      {{if eq .SendMsg true}}
        document.getElementById("{{ .Name }}").checked = true;
      {{end}}
    {{ end }}

    {{ if eq .CurrentBroadcast.SendMsg true}}
      document.getElementById("report-sensor").checked = true;
    {{end}}
  }

  function checkAll(form){
    {{ range .CurrentBroadcast.SensorList }}
      form.querySelector("input[id='{{ .Name }}']").checked = true;
    {{ end }}
    form.submit()
  }

  function uncheckAll(form){
    {{ range .CurrentBroadcast.SensorList }}
      form.querySelector("input[id='{{ .Name }}']").checked = false;
    {{ end }}
    form.submit()
  }

  function buttonClick(button){
    button.form.querySelector("input[name='action']").value = button.value
    button.form.submit()
  }

  function submitSelect(select){
    select.form.querySelector("input[name='action']").value = "broadcast-select"
    select.form.submit();
  }
