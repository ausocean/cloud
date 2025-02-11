async function handleFormSubmit(event: Event): Promise<void> {
  console.log("handling form submission...");
  event.preventDefault();

  const city = (document.querySelector("#city") as HTMLSelectElement).value;
  const interest = (document.querySelector("#user-category") as HTMLSelectElement).value;

  try {
    const response = await fetch("/api/v1/survey", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ city, "user-category": interest }).toString(),
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
  