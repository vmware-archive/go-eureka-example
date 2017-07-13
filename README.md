## Eureka go demo
This repository demonstrates how to use Eureka in conjunction with go applications.
There are two apps, frontend and backend. The frontend is able to discover the backend
using Eureka and forward traffic to it over the container network.

## Requirements
- cf CLI
- cf CLI `network-policy` plugin
- PWS account

## How-To
0. Log in to PWS
1. Create and target a space
2. Create an instance of the Spring Cloud Services Registry:
```
cf create-service p-service-registry standard scs-registry
```

3. Deploy (but don't start) the backend application
```
cd backend
cf push cats-backend --no-start
```

3. Deploy (but don't start) the frontend application
```
cd ../frontend
cf push cats-frontend --no-start
```

4. Bind both the frontend and backend to the scs-registry
```
cf bind-service cats-backend scs-registry
cf bind-service cats-frontend scs-registry
```

5. Start the apps
```
cf start cats-frontend
cf start cats-backend
```

6. Add a policy to allow access over the container network
```
cf allow-access cats-frontend cats-backend --port 7007 --protocol tcp
```

6. Go to the url of the cats-frontend app (in this example `cats-frontend.run.pivotal.io`)

7. Type `cats-backend` into the field for appName

8. Hit submit and see that you are correctly served a cat. Multiple repeats should result in different backend ips being presented.
