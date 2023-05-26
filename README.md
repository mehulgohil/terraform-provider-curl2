# terraform-provider-curl2

## Overview
terraform-provider-curl2 is designed to help make HTTP(s) requests,
with additional support for providing JSON bodies and authentication headers.

Key Features:

HTTP Method Support:
The custom provider allows you to perform various HTTP methods like GET, POST, PUT, DELETE, etc., using cURL commands.
JSON Body Support: You can provide JSON payloads as the request body for methods like POST or PUT.
This enables you to send structured data to the API endpoints.
Authentication Headers: The custom provider supports the inclusion of authentication headers in the HTTP requests.
You can provide headers like API keys, tokens, or other authentication mechanisms required by the API.