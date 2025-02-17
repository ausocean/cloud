/// <reference types="google.maps" />

async function handleFormSubmit(event: Event): Promise<void> {
  console.log("handling form submission...");
  event.preventDefault();

  const regionInput = (document.querySelector("#region") as HTMLInputElement).value;
  const userCategory = (document.querySelector("#user-category") as HTMLSelectElement).value;

  try {
    const response = await fetch("/api/v1/survey", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        region: regionInput,
        "user-category": userCategory,
      }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw response.statusText + ": " + error.message;
    } else {
      window.location.href = "/watch.html";
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
    console.warn("form element not found");
  }
}

// Initialize the form submission handler when the document is ready.
document.addEventListener("DOMContentLoaded", initFormHandler);

function initAutocomplete(): void {
  const input = document.getElementById("location") as HTMLInputElement;
  const regionInput = document.getElementById("region") as HTMLInputElement;

  if (!input || !regionInput) {
    console.error("location or region input not found");
    return;
  }

  const autocomplete = new google.maps.places.Autocomplete(input, {
    types: ["(regions)"],
    componentRestrictions: { country: "AU" },
  });

  // Remove AU restriction after 5+ characters to allow global search.
  input.addEventListener("input", () => {
    if (input.value.length >= 5) {
      autocomplete.setComponentRestrictions({ country: [] });
    }
  });

  autocomplete.addListener("place_changed", () => {
    const place = autocomplete.getPlace();

    if (!place.address_components) {
      console.warn("no address components found.");
      return;
    }

    const regionData: Record<string, string> = {};

    for (const component of place.address_components) {
      const types = component.types;
      for (const type of types) {
        regionData[type] = component.long_name;
      }
    }

    regionInput.value = JSON.stringify(regionData);
    console.log("Region Data:", regionData);
  });
}

document.addEventListener("DOMContentLoaded", initAutocomplete);
