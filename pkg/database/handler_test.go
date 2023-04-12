package database

//TODO: !!!
/*
func TestHandler_Upload(t *testing.T) {
	userClient := &mockUserClient{}
	userClient.
		On("Find", "token", "group-name").
		Return(&model.Group{Name: "group-name"}, nil)
	s3Uploader := &mockAwsS3Uploader{}
	putObjectInput := mock.MatchedBy(func(put *s3.PutObjectInput) bool {
		body := new(strings.Builder)
		_, err := io.Copy(body, put.Body)
		if err != nil {
			t.Fatalf("failed to copy body: %v", err)
		}
		return *put.Bucket == *aws.String("") &&
			*put.Key == *aws.String("group-name/database.sql") &&
			body.String() == "Hello, World!"
	})
	s3Uploader.
		On("Upload", mock.AnythingOfType("*context.emptyCtx"), putObjectInput, mock.AnythingOfType("[]func(*manager.Uploader)")).
		Return(&manager.UploadOutput{}, nil)
	s3Client := storage.NewS3Client(nil, s3Uploader)
	repository := &mockRepository{}
	repository.
		On("Save", mock.AnythingOfType("*model.Database")).
		Return(nil)
	service := NewService(config.Config{}, nil, s3Client, repository)
	handler := New(userClient, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.Request = newMultipartRequest(t, "group-name", "database.sql", "Hello, World!")

	handler.Upload(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusCreated, &model.Database{Name: "database.sql", GroupName: "group-name", Url: "s3:///group-name/database.sql"})
	repository.AssertExpectations(t)
	s3Uploader.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func newMultipartRequest(t *testing.T, group string, filename string, fileContent string) *http.Request {
	var buf bytes.Buffer
	multipartWriter := multipart.NewWriter(&buf)
	err := multipartWriter.WriteField("group", group)
	require.NoError(t, err)
	filePart, err := multipartWriter.CreateFormFile("database", filename)
	require.NoError(t, err)
	_, err = filePart.Write([]byte(fileContent))
	require.NoError(t, err)
	err = multipartWriter.Close()
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "", &buf)
	require.NoError(t, err)
	request.Header.Set("Authorization", "token")
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("Content-Length", strconv.Itoa(len(fileContent)))
	return request
}

type mockAwsS3Uploader struct{ mock.Mock }

func (m *mockAwsS3Uploader) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	called := m.Called(ctx, input, opts)
	return called.Get(0).(*manager.UploadOutput), nil
}

func TestHandler_ExternalDownload(t *testing.T) {
	awsS3Client := &mockAWSS3Client{}
	awsS3Client.
		On("GetObject", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*s3.GetObjectInput"), mock.AnythingOfType("[]func(*s3.Options)")).
		Return(&s3.GetObjectOutput{
			Body:          io.NopCloser(strings.NewReader("Hello, World!")),
			ContentLength: 13,
		}, nil)
	s3Client := storage.NewS3Client(awsS3Client, nil)
	database := &model.Database{
		Model:     gorm.Model{ID: 1},
		GroupName: "group-name",
		Url:       "s3://whatever",
	}
	id := uuid.New()
	repository := &mockRepository{}
	repository.
		On("FindExternalDownload", id).
		Return(model.ExternalDownload{
			DatabaseID: 1,
		}, nil)
	repository.
		On("FindById", uint(1)).
		Return(database, nil)
	repository.
		On("PurgeExternalDownload").
		Return(nil)
	service := NewService(config.Config{}, nil, s3Client, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("uuid", id.String())

	handler.ExternalDownload(c)

	assert.Empty(t, c.Errors)
	headers := w.Header()
	assert.Equal(t, "attachment; filename=whatever", headers.Get("Content-Disposition"))
	assert.Equal(t, "File Transfer", headers.Get("Content-Description"))
	assert.Equal(t, "binary", headers.Get("Content-Transfer-Encoding"))
	assert.Equal(t, "application/octet-stream", headers.Get("Content-Type"))
	assert.Equal(t, "13", headers.Get("Content-Length"))
	assert.Equal(t, "Hello, World!", w.Body.String())
	repository.AssertExpectations(t)
	awsS3Client.AssertExpectations(t)
}

func TestHandler_CreateExternalDownload(t *testing.T) {
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(&model.Database{
			Model:     gorm.Model{ID: 1},
			GroupName: "group-name",
		}, nil)
	repository.
		On("PurgeExternalDownload").
		Return(nil)
	expiration := time.Now().Add(time.Duration(1) * time.Hour).Round(time.Duration(1)).UTC()
	externalDownload := model.ExternalDownload{
		UUID:       uuid.UUID{},
		Expiration: expiration,
		DatabaseID: 1,
	}
	repository.
		On("CreateExternalDownload", uint(1), expiration).
		Return(externalDownload, nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")
	createExternalDatabaseRequest := &CreateExternalDatabaseRequest{Expiration: expiration}
	c.Request = newPost(t, "/databases/1/external", createExternalDatabaseRequest)

	handler.CreateExternalDownload(c)

	require.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusCreated, &externalDownload)
	repository.AssertExpectations(t)
}

func TestHandler_Download(t *testing.T) {
	awsS3Client := &mockAWSS3Client{}
	awsS3Client.
		On("GetObject", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*s3.GetObjectInput"), mock.AnythingOfType("[]func(*s3.Options)")).
		Return(&s3.GetObjectOutput{
			Body:          io.NopCloser(strings.NewReader("Hello, World!")),
			ContentLength: 13,
		}, nil)
	s3Client := storage.NewS3Client(awsS3Client, nil)
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(&model.Database{
			Model:     gorm.Model{ID: 1},
			GroupName: "group-name",
			Url:       "s3://whatever",
		}, nil)
	service := NewService(config.Config{}, nil, s3Client, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")

	handler.Download(c)

	assert.Empty(t, c.Errors)
	headers := w.Header()
	assert.Equal(t, "attachment; filename=whatever", headers.Get("Content-Disposition"))
	assert.Equal(t, "File Transfer", headers.Get("Content-Description"))
	assert.Equal(t, "binary", headers.Get("Content-Transfer-Encoding"))
	assert.Equal(t, "application/octet-stream", headers.Get("Content-Type"))
	assert.Equal(t, "13", headers.Get("Content-Length"))
	assert.Equal(t, "Hello, World!", w.Body.String())
	repository.AssertExpectations(t)
	awsS3Client.AssertExpectations(t)
}

type mockAWSS3Client struct{ mock.Mock }

func (m *mockAWSS3Client) CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	called := m.Called(ctx, params, optFns)
	return called.Get(0).(*s3.CopyObjectOutput), nil
}

func (m *mockAWSS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	called := m.Called(ctx, params, optFns)
	return called.Get(0).(*s3.DeleteObjectOutput), nil
}

func (m *mockAWSS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	called := m.Called(ctx, params, optFns)
	return called.Get(0).(*s3.GetObjectOutput), nil
}

func TestHandler_Update(t *testing.T) {
	database := &model.Database{GroupName: "group-name"}
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(database, nil)
	repository.
		On("Update", database).
		Return(nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")
	updateDatabaseRequest := &UpdateDatabaseRequest{Name: "database-name"}
	c.Request = newPost(t, "/databases/1", updateDatabaseRequest)

	handler.Update(c)

	assert.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, database)
	repository.AssertExpectations(t)
}

func TestHandler_Unlock(t *testing.T) {
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(&model.Database{
			GroupName: "group-name",
			Lock: &model.Lock{
				DatabaseID: 1,
				InstanceID: 1,
				UserID:     1,
			},
		}, nil)
	repository.
		On("Unlock", uint(1)).
		Return(nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")

	handler.Unlock(c)

	assert.Empty(t, c.Errors)
	assert.Empty(t, w.Body)
	c.Writer.Flush()
	assert.Equal(t, http.StatusAccepted, w.Code)
	repository.AssertExpectations(t)
}

func TestHandler_Lock(t *testing.T) {
	repository := &mockRepository{}
	database := &model.Database{GroupName: "group-name"}
	repository.
		On("FindById", uint(1)).
		Return(database, nil)
	lock := &model.Lock{
		DatabaseID: 1,
		InstanceID: 1,
		UserID:     1,
	}
	repository.
		On("Lock", uint(1), uint(1), uint(1)).
		Return(lock, nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")
	lockDatabaseRequest := &LockDatabaseRequest{InstanceId: 1}
	c.Request = newPost(t, "/whatever", lockDatabaseRequest)

	handler.Lock(c)

	assert.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusCreated, lock)
	repository.AssertExpectations(t)
}

func TestHandler_Delete(t *testing.T) {
	awsS3Client := &mockAWSS3Client{}
	awsS3Client.
		On("DeleteObject", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*s3.DeleteObjectInput"), mock.AnythingOfType("[]func(*s3.Options)")).
		Return(&s3.DeleteObjectOutput{})
	s3Client := storage.NewS3Client(awsS3Client, nil)
	database := &model.Database{
		GroupName: "group-name",
		Url:       "/path",
	}
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(database, nil)
	repository.
		On("Delete", uint(1)).
		Return(nil)
	service := NewService(config.Config{}, nil, s3Client, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")

	handler.Delete(c)

	assert.Empty(t, c.Errors)
	assert.Empty(t, w.Body)
	c.Writer.Flush()
	assert.Equal(t, http.StatusAccepted, w.Code)
	repository.AssertExpectations(t)
	awsS3Client.AssertExpectations(t)
}

func newContext(w *httptest.ResponseRecorder, group string) *gin.Context {
	user := &model.User{
		ID: uint64(1),
		Groups: []*model.Group{
			{Name: group},
		},
	}
	c, _ := gin.CreateTestContext(w)
	c.Set("user", user)
	return c
}

func TestHandler_FindByIdentifier(t *testing.T) {
	repository := &mockRepository{}
	database := &model.Database{
		GroupName: "group-name",
	}
	repository.
		On("FindById", uint(1)).
		Return(database, nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")

	handler.FindByIdentifier(c)

	assert.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, database)
	repository.AssertExpectations(t)
}

func TestHandler_FindByIdentifier_Slug(t *testing.T) {
	repository := &mockRepository{}
	database := &model.Database{
		Model:     gorm.Model{ID: 1},
		GroupName: "group-name",
	}
	repository.
		On("FindBySlug", "slug").
		Return(database, nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "slug")

	handler.FindByIdentifier(c)

	assert.Empty(t, c.Errors)
	assertResponse(t, w, http.StatusOK, database)
	repository.AssertExpectations(t)
}

func TestHandler_Copy(t *testing.T) {
	userClient := &mockUserClient{}
	userClient.
		On("Find", "token", "group-name").
		Return(&model.Group{
			Name: "group-name",
		}, nil)
	awsS3Client := &mockAWSS3Client{}
	awsS3Client.
		On("CopyObject", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*s3.CopyObjectInput"), mock.AnythingOfType("[]func(*s3.Options)")).
		Return(&s3.CopyObjectOutput{}, nil)
	s3Client := storage.NewS3Client(awsS3Client, nil)
	repository := &mockRepository{}
	repository.
		On("FindById", uint(1)).
		Return(&model.Database{
			Url: "/path",
		}, nil)
	repository.
		On("Create", mock.AnythingOfType("*model.Database")).
		Return(nil)
	service := NewService(config.Config{}, nil, s3Client, repository)
	handler := New(userClient, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")
	c.AddParam("id", "1")
	copyRequest := &CopyDatabaseRequest{
		Name:  "database-name",
		Group: "group-name",
	}
	c.Request = newPost(t, "/groups", copyRequest)

	handler.Copy(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Empty(t, c.Errors)
	userClient.AssertExpectations(t)
	awsS3Client.AssertExpectations(t)
	repository.AssertExpectations(t)
}

func newPost(t *testing.T, path string, jsonBody any) *http.Request {
	body, err := json.Marshal(jsonBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Authorization", "token")

	return req
}

func TestHandler_List(t *testing.T) {
	databases := []*model.Database{
		{
			Model:     gorm.Model{ID: 1},
			Name:      "some name",
			GroupName: "group-name",
			Url:       "",
		},
	}
	repository := &mockRepository{}
	repository.
		On("FindByGroupNames", []string{"group-name"}).
		Return(databases, nil)
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")

	handler.List(c)

	assert.Empty(t, c.Errors)
	expectedBody := &[]GroupsWithDatabases{
		{
			Name:      "group-name",
			Databases: databases,
		},
	}
	assertResponse(t, w, http.StatusOK, expectedBody)
	repository.AssertExpectations(t)
}

func assertResponse[V any](t *testing.T, rec *httptest.ResponseRecorder, expectedCode int, expectedBody V) {
	require.Equal(t, expectedCode, rec.Code, "HTTP status code does not match")
	assertJSON(t, rec.Body, expectedBody)
}

func assertJSON[V any](t *testing.T, body *bytes.Buffer, expected V) {
	actualBody := new(V)
	err := json.Unmarshal(body.Bytes(), &actualBody)
	require.NoError(t, err)
	require.Equal(t, expected, *actualBody, "HTTP response body does not match")
}

func TestHandler_List_RepositoryError(t *testing.T) {
	repository := &mockRepository{}
	repository.
		On("FindByGroupNames", []string{"group-name"}).
		Return(nil, errors.New("some error"))
	service := NewService(config.Config{}, nil, nil, repository)
	handler := New(nil, service, nil, nil)

	w := httptest.NewRecorder()
	c := newContext(w, "group-name")

	handler.List(c)

	assert.Empty(t, w.Body.Bytes())
	assert.Len(t, c.Errors, 1)
	assert.ErrorContains(t, c.Errors[0].Err, "some error")
	repository.AssertExpectations(t)
}

type mockRepository struct{ mock.Mock }

func (m *mockRepository) Create(d *model.Database) error {
	return m.Called(d).Error(0)
}

func (m *mockRepository) Save(d *model.Database) error {
	return m.Called(d).Error(0)
}

func (m *mockRepository) FindById(id uint) (*model.Database, error) {
	called := m.Called(id)
	database, ok := called.Get(0).(*model.Database)
	if ok {
		return database, nil
	} else {
		return nil, called.Error(1)
	}
}

func (m *mockRepository) FindBySlug(slug string) (*model.Database, error) {
	called := m.Called(slug)
	database, ok := called.Get(0).(*model.Database)
	if ok {
		return database, nil
	} else {
		return nil, called.Error(1)
	}
}

func (m *mockRepository) Lock(id, instanceId, userId uint) (*model.Lock, error) {
	called := m.Called(id, instanceId, userId)
	return called.Get(0).(*model.Lock), nil
}

func (m *mockRepository) Unlock(id uint) error {
	called := m.Called(id)
	return called.Error(0)
}

func (m *mockRepository) Delete(id uint) error {
	return m.Called(id).Error(0)
}

func (m *mockRepository) FindByGroupNames(names []string) ([]model.Database, error) {
	called := m.Called(names)
	databases, ok := called.Get(0).([]model.Database)
	if ok {
		return databases, nil
	}
	return nil, called.Error(1)
}

func (m *mockRepository) Update(d *model.Database) error {
	called := m.Called(d)
	return called.Error(0)
}

func (m *mockRepository) CreateExternalDownload(databaseID uint, expiration time.Time) (model.ExternalDownload, error) {
	called := m.Called(databaseID, expiration)
	return called.Get(0).(model.ExternalDownload), nil
}

func (m *mockRepository) FindExternalDownload(uuid uuid.UUID) (model.ExternalDownload, error) {
	called := m.Called(uuid)
	return called.Get(0).(model.ExternalDownload), nil
}

func (m *mockRepository) PurgeExternalDownload() error {
	return m.Called().Error(0)
}

type mockUserClient struct{ mock.Mock }

func (m *mockUserClient) FindGroupByName(token string, name string) (*model.Group, error) {
	called := m.Called(token, name)
	return called.Get(0).(*model.Group), nil
}

func (m *mockUserClient) FindUserById(token string, id uint) (*model.User, error) {
	panic("implement me")
}
*/
