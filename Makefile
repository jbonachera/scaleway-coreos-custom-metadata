build::
	docker build -t jbonachera/scaleway-coreos-custom-metadata .
	docker create --name artifacts jbonachera/scaleway-coreos-custom-metadata
	docker cp artifacts:/bin/scaleway-coreos-custom-metadata scaleway-coreos-custom-metadata
	docker rm artifacts

