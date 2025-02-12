// This script checks if the user has completed the survey and redirects them to the survey page if they haven't.
async function checkSurveyRedirect() {
  try {
    const response = await fetch("/api/v1/survey/check", { credentials: "include" });
    if (!response.ok) {
      const error = await response.json();
      throw response.statusText + ": " + error.message;
    }
    const result = await response.json();
    if (result.redirect) {
      window.location.href = result.redirect;
    }
  } catch (error) {
    console.error("Failed to check survey redirect:", error);
  }
}

checkSurveyRedirect();
