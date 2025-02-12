async function handleFormSubmit(event: Event): Promise<void> {
  console.log("handling form submission...");
  event.preventDefault();

  const city = (document.querySelector("#city") as HTMLSelectElement).value;
  const postcode = (document.querySelector("#postcode") as HTMLSelectElement).value;
  const userCategory = (document.querySelector("#user-category") as HTMLSelectElement).value;

  try {
    const response = await fetch("/api/v1/survey", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ city: city, postcode: postcode, "user-category": userCategory }).toString(),
    });

    if (!response.ok) {
      const error = await response.json();
      alert(`Error: ${error.error}`);
    } else {
      window.location.href = "/watch.html"; // Redirect to home page.
    }
  } catch (error) {
    console.error("error submitting survey form:", error);
    alert("An unexpected error occurred. Please contact us or try again later.");
  }
}

function initFormHandler(): void {
  console.log("initializing form handler...");
  const form = document.querySelector("form") as HTMLFormElement | null;
  if (form) {
    form.addEventListener("submit", handleFormSubmit);
  } else {
    console.warn("form element not found!");
  }
}

// Initialize the form submission handler when the document is ready.
document.addEventListener("DOMContentLoaded", initFormHandler);

function initAutocomplete(): void {
  const input = document.getElementById("location") as HTMLInputElement;
  const cityInput = document.getElementById("city") as HTMLInputElement;
  const postcodeInput = document.getElementById("postcode") as HTMLInputElement;

  if (!input) {
    console.error("location input not found");
    return;
  }

  const autocomplete = new google.maps.places.Autocomplete(input, {
    types: ["geocode"], // Prioritize city/postcode addresses.
  });

  autocomplete.addListener("place_changed", () => {
    const place = autocomplete.getPlace();

    if (!place.address_components) {
      console.warn("no address components found.");
      return;
    }

    let city = "";
    let postcode = "";

    for (const component of place.address_components) {
      if (component.types.includes("locality")) {
        city = component.long_name;
      }
      if (component.types.includes("postal_code")) {
        postcode = component.long_name;
      }
    }

    // Fill the hidden fields.
    cityInput.value = city;
    postcodeInput.value = postcode;
  });
}

document.addEventListener("DOMContentLoaded", initAutocomplete);
