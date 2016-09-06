
#client add [id]                  Add client registration
#                                   --name <string>
#                                   --scope <list>
#                                   --authorized_grant_types <list>
#                                   --authorities <list>
#                                   --access_token_validity <seconds>
#                                   --refresh_token_validity <seconds>
#                                   --redirect_uri <list>
#                                   --autoapprove <list>
#                                   --signup_redirect_url <url>
#                                   --clone <other>, get default settings from other
#                                   -s | --secret <secret>, client secret
#                                   -i | --[no-]interactive, interactively verify all values

# Datadog example
# properties:
#   uaa:
#     clients:
#       datadog-firehose-nozzle:
#         access-token-validity: 1209600
#         authorized-grant-types: authorization_code,client_credentials,refresh_token
#         override: true
#         secret: <password>
#         scope: openid,oauth.approvals,doppler.firehose
#         authorities: oauth.login,doppler.firehose

#uac mple-nozzle:
#        access-token-validity: 1209600
#        authorized-grant-types: authorization_code,client_credentials,refresh_token
#        override: true
#        secret: example-nozzle
#        scope: openid,oauth.approvals,doppler.firehose
#        authorities: oauth.login,doppler.firehose

uaac client add oms-nozzle \
--name oms-nozzle \
--scope openid,oauth.approvals,doppler.firehose \
--authorized_grant_types authorization_code,client_credentials,refresh_token \
--authorities oauth.login,doppler.firehose \
--access_token_validity  31557600 \
--refresh_token_validity 31557600 

