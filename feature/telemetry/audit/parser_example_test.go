package audit

import (
	"fmt"
	"github.com/9edang/inkgate/internal/json"

	"github.com/9edang/inkgate/engine/config"
)

func ExampleParse() {
	cfg, err := config.NewParser().Parse("./tests/example1.json")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	cfg.Normalize()

	result := Parse(&cfg)
	r, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(r))

	// output:
	// {
	//   "d": [
	//     7220
	//   ],
	//   "a": null,
	//   "e": [
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         140000,
	//         0,
	//         0
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "inkgate/feature/traffic/httpcache": [
	//               0
	//             ],
	//             "inkgate/feature/transform/lua/proxy/backend": [
	//               2
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {
	//         "inkgate/feature/auth/jose/validator": [],
	//         "inkgate/feature/transform/lua/proxy": [
	//           3
	//         ],
	//         "modifier/response-body": [
	//           5,
	//           2,
	//           0,
	//           1,
	//           1,
	//           1
	//         ],
	//         "validation/response-json-schema": [
	//           18,
	//           1,
	//           400,
	//           1
	//         ]
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         1,
	//         1,
	//         10000,
	//         7,
	//         0
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "backend/http/client": [
	//               3
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {
	//         "inkgate/engine/transport/http/client/plugin": [
	//           1
	//         ]
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         0
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {
	//         "websocket": [
	//           27,
	//           4096,
	//           4096,
	//           4096,
	//           3200000,
	//           0,
	//           10000,
	//           60000,
	//           54000,
	//           300000,
	//           1
	//         ]
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         0
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "inkgate/feature/traffic/httpcache": [
	//               7
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {}
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "ai/llm": [
	//               1,
	//               0,
	//               0,
	//               1
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {}
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "ai/llm": [
	//               2,
	//               1,
	//               0,
	//               1
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {}
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "ai/llm": [
	//               4,
	//               0,
	//               1,
	//               1
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {}
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {
	//             "ai/llm": [
	//               8,
	//               1,
	//               1,
	//               1
	//             ]
	//           }
	//         }
	//       ],
	//       "c": {}
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {
	//         "ai/mcp": []
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         0
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {
	//         "ai/mcp": []
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {
	//         "ai/mcp": []
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         1
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {
	//         "ai/mcp": []
	//       }
	//     },
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         10000,
	//         8,
	//         2
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         },
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         },
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {
	//         "inkgate/engine/proxy": [
	//           1
	//         ]
	//       }
	//     }
	//   ],
	//   "c": {
	//     "ai/mcp": [
	//       2,
	//       3
	//     ],
	//     "auth/api-keys": [],
	//     "grpc": [
	//       1
	//     ],
	//     "inkgate/engine/router": [
	//       262144
	//     ],
	//     "inkgate/engine/transport/http/server/plugin": [
	//       4
	//     ],
	//     "inkgate/feature/auth/bloomfilter": [
	//       1,
	//       2,
	//       0
	//     ],
	//     "inkgate/feature/telemetry/influx": [],
	//     "inkgate/feature/transform/lua/router": [
	//       1
	//     ],
	//     "modifier/response-headers": [
	//       15
	//     ],
	//     "qos/ratelimit/service": [],
	//     "telemetry/opentelemetry": [
	//       50,
	//       100,
	//       1,
	//       2,
	//       1
	//     ]
	//   }
	// }
}

func ExampleParse_withRevokerServer() {
	cfg, err := config.NewParser().Parse("./tests/revoker.json")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	cfg.Normalize()

	result := Parse(&cfg)
	r, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(r))

	// output:
	// {
	//   "d": [
	//     0
	//   ],
	//   "a": null,
	//   "e": [
	//     {
	//       "d": [
	//         2,
	//         0,
	//         0,
	//         2000,
	//         0,
	//         0
	//       ],
	//       "b": [
	//         {
	//           "d": [
	//             64
	//           ],
	//           "c": {}
	//         }
	//       ],
	//       "c": {}
	//     }
	//   ],
	//   "c": {
	//     "inkgate/feature/auth/bloomfilter": [
	//       1,
	//       1,
	//       1
	//     ]
	//   }
	// }
}
