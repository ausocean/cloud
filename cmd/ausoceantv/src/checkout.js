// This is your test publishable API key.
const stripe = Stripe(import.meta.env.VITE_STRIPE_PUBLIC_KEY);

let elements;

initialize();

document
  .querySelector("#payment-form")
  .addEventListener("submit", handleSubmit);

// Fetches a payment intent and captures the client secret
async function initialize() {
  // Extract the 'priceID' query parameter from the URL
  const urlParams = new URLSearchParams(window.location.search);
  const id = urlParams.get("priceID");

  if (!id) {
    console.log("setting error message");
    let msg = document.getElementById("msg");
    msg.innerHTML = "<p>Choose a Plan <a href='/plans.html'>here</a></p>";
    msg.removeAttribute("hidden");

    document.getElementById("payment-form").setAttribute("hidden", true);
    return;
  }

  const response = await fetch(
    "/api/v1/stripe/create-payment-intent?priceID=" + id,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    },
  );
  const { clientSecret, dpmCheckerLink } = await response.json();

  const appearance = {
    theme: "stripe",
  };
  elements = stripe.elements({ appearance, clientSecret });

  const paymentElementOptions = {
    layout: "tabs",
  };

  const paymentElement = elements.create("payment", paymentElementOptions);
  paymentElement.mount("#payment-element");

  const priceJSON = await fetch("api/v1/stripe/price/" + id, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  }).then((resp) => {
    return resp.json();
  });
  console.debug(priceJSON);

  const product = await fetch("api/v1/stripe/product/" + priceJSON.product.id, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  }).then((resp) => {
    return resp.json();
  });
  console.debug(product);

  // Show the product information in the cart.
  let items = document.getElementById("items");
  let name = document.getElementById("name");
  let price = document.getElementById("price");
  let desc = document.getElementById("desc");

  name.innerText = product.name;
  price.innerText = "$" + priceJSON.unit_amount / 100;
  desc.innerText = product.description;

  // Stop the loading animation.
  items.classList.remove("animate-pulse");

  // [DEV] For demo purposes only
  setDpmCheckerLink(dpmCheckerLink);
}

async function handleSubmit(e) {
  e.preventDefault();
  setLoading(true);

  const { error } = await stripe.confirmPayment({
    elements,
    confirmParams: {
      // Make sure to change this to your payment completion page
      return_url: "http://localhost:5173/complete.html",
    },
  });

  // This point will only be reached if there is an immediate error when
  // confirming the payment. Otherwise, your customer will be redirected to
  // your `return_url`. For some payment methods like iDEAL, your customer will
  // be redirected to an intermediate site first to authorize the payment, then
  // redirected to the `return_url`.
  if (error.type === "card_error" || error.type === "validation_error") {
    showMessage(error.message);
  } else {
    showMessage("An unexpected error occurred.");
  }

  setLoading(false);
}

// ------- UI helpers -------

function showMessage(messageText) {
  const messageContainer = document.querySelector("#payment-message");

  messageContainer.classList.remove("hidden");
  messageContainer.textContent = messageText;

  setTimeout(function () {
    messageContainer.classList.add("hidden");
    messageContainer.textContent = "";
  }, 4000);
}

// Show a spinner on payment submission
function setLoading(isLoading) {
  if (isLoading) {
    // Disable the button and show a spinner
    document.querySelector("#submit").disabled = true;
    document.querySelector("#spinner").classList.remove("hidden");
    document.querySelector("#button-text").classList.add("hidden");
  } else {
    document.querySelector("#submit").disabled = false;
    document.querySelector("#spinner").classList.add("hidden");
    document.querySelector("#button-text").classList.remove("hidden");
  }
}

function setDpmCheckerLink(url) {
  document.querySelector("#dpm-integration-checker").href = url;
}
