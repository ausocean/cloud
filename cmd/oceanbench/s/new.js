var buttons;
var type;

function init() {
	buttons = document.getElementById("types").children;
}

function setType(value) {
	type = value;
	for (let button of buttons) {
		if (button.value == value) {
			button.classList.add("btn-primary");
			button.classList.remove("btn-outline-primary");
		} else {
			button.classList.remove("btn-primary");
			button.classList.add("btn-outline-primary");
		}
	};

	toggleModules();
}

function toggleModules() {
	let controllerParts = document.getElementsByClassName("controller");
	let error = document.getElementById("unimplemented");
	switch (type) {
		case "Controller":
			error.classList.add("d-none");
			for (let part of controllerParts) {
				part.classList.remove("d-none");
			}
			break;
		default:
			error.classList.remove("d-none");
			for (let part of controllerParts) {
				part.classList.add("d-none");
			}
			break;
	}
}