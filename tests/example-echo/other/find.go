package other

type FindResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func Find(t int) (int, []FindResponse, FindResponse, error) {
	if t == 1 {
		return 1, []FindResponse{
			{ID: 1, Name: "example1"},
			{ID: 2, Name: "example2"},
		}, FindResponse{}, nil
	}

	return 2, []FindResponse{
		{ID: 4, Name: "example4"},
		{ID: 5, Name: "example5"},
	}, FindResponse{ID: 6, Name: "example6"}, nil
}
