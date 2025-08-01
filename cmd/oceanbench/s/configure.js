function init() {
  // Fetch all the forms we want to apply custom Bootstrap validation styles to
  const forms = document.querySelectorAll(".needs-validation");

  // Loop over them and prevent submission
  Array.from(forms).forEach((form) => {
    form.addEventListener(
      "submit",
      async (event) => {
        // Run custom Bootstrap validation
        if (!form.checkValidity()) {
          event.preventDefault(); // Only prevent submission if validation fails
          event.stopPropagation();
          form.classList.add("was-validated");
          return;
        }
      },
      false,
    );
  });
}
