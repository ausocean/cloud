async function checkSurveyRedirect() {
  try {
    const response = await fetch("/api/v1/survey/check", { credentials: "include" });
    const result = await response.json();
    if (result.redirect && result.redirect !== "none") {
      window.location.href = result.redirect;
    }
  } catch (error) {
    console.error("Failed to check survey redirect:", error);
  }
}

checkSurveyRedirect();
