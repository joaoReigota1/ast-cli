#!/bin/bash
for ARGUMENT in "$@"
do
  KEY=$(echo $ARGUMENT | cut -f1 -d=)
    VALUE=$(echo $ARGUMENT | cut -f2 -d=)
    case "$KEY" in
            log_rotation_size)              log_rotation_size=${VALUE} ;;
            log_rotation_age_days)    log_rotation_age_days=${VALUE} ;;
            private_key_path)    private_key_path=${VALUE} ;;
            certificate_path)    certificate_path=${VALUE} ;;
            fqdn)    fqdn=${VALUE} ;;
          deploy_DB)    deploy_DB=${VALUE} ;;
          deploy_TLS)    deploy_TLS=${VALUE} ;;
            *)
    esac
done
echo $log_rotation_size#$log_rotation_age_days#$private_key_path#$certificate_path#$fqdn#$(( deploy_DB == true ))#$(( deploy_TLS == true ))