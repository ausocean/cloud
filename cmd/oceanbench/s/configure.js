function init() {
  // Fetch all the forms we want to apply custom Bootstrap validation styles to
  const forms = document.querySelectorAll(".needs-validation");

  // Loop over them and prevent submission
  Array.from(forms).forEach((form) => {
    form.addEventListener(
      "submit",
      async (event) => {
        event.preventDefault();
        event.stopPropagation();
        if (!form.checkValidity()) {
          form.classList.add("was-validated");
          return;
        }

        // Format the request to meet the REST API.
        let data = new FormData(form);

        if (data.get("lat") != "" && data.get("long") != "") {
          data.set("ll", data.get("lat") + "," + data.get("long"));
        }
        data.delete("lat");
        data.delete("long");

        if (data.get("ssid") != "" && data.get("pass") != "") {
          data.set("wi", data.get("ssid") + "," + data.get("pass"));
        }
        data.delete("ssid");
        data.delete("pass");

        await submitForm(data);
      },
      false,
    );
  });
}

async function submitForm(data) {
  // Submit the form.
  const resp = await fetch("/set/devices/configure", {
    method: "POST",
    body: data,
  });

  if (resp.redirected) {
    console.log("redirecting to devices page");
    window.location.href = resp.url;
    return;
  }

  const error = await resp.json();

  let msg = document.getElementById("msg");
  msg.innerText = error.er;
  msg.style.display = "block";
}
