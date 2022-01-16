#!/bin/sh

cat << EOF > /config.json
{
    "Type":"Group",
    "Services":{
        "agent":{
            "Type":"Agent",
            "Agent":"agent://server.agent"
        },
        "accept":{
            "Type":"Accept",
            "Agent":"agent://server.agent",
            "Listen":"tcp://0.0.0.0:${PORT}"
        }
    }
}
EOF

./bp