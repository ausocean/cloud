# AusOcean Roadmap

This directory contains the backend and frontend for the AusOcean Roadmap web application.

## Local Development

To run the application locally, you will need to run the frontend development server and the backend Go server concurrently in separate terminals.

### 1. Frontend Setup

Navigate to the `webapp` directory:

```bash
cd webapp
```

**Important**: Make sure your `webapp/.env` file is pointed at your local Go server by adding or setting `VITE_API_URL=http://localhost:8080`. Otherwise, your local frontend will likely fetch data from the live production server.


Install the dependencies, build the project, and start the development server:

```bash
npm install
npm run build
npm run dev
```

### 2. Backend Setup

Open a new terminal session in the root of the backend (`cmd/roadmap`) directory.

For local development with authentication, you must export the `OAUTH2_CALLBACK` environment variable before running the backend:

```bash
export OAUTH2_CALLBACK=http://localhost:8080/api/v1/auth/oauth2callback
```

Build and run the Go application:

```bash
go build
./roadmap
```

The backend server should now be running on port `8080` and the frontend should be accessible via Vite (usually `http://localhost:5173`).
