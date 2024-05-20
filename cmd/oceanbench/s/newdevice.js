window.onload = () => {
	document.getElementById("type").addEventListener("change", showWifi);
	showWifi();
};

function showWifi(Event) {
	let wifi = document.getElementById("wifi-group");
	console.log(wifi);
	console.log(Event.target);
	let type = Event.target.value;

	console.log("checking wifi showing, type:", type);
	switch (type) {
		case "Controller":
			console.log("class list before:", wifi.classList);
			wifi.classList.remove("d-none");
			wifi.classList.add("d-flex");
			console.log("class list after:", wifi.classList);
			break;

		default:
			console.log("class list before:", wifi.classList);
			wifi.classList.remove("d-flex");
			wifi.classList.add("d-none");
			console.log("hiding wifi");
			console.log("class list after:", wifi.classList);
			break;
	}
}