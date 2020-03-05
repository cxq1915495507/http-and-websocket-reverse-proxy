<?php
		$data = $_POST["action"].'';
		$data = '{"Cmd":"devList","Data":"aaa"}'."\n";
		error_log($data);
		$client = stream_socket_client("unix:///dev/shm/unixsock", $errno, $errstr);

		if (!$client)
		{
			die("connect to server fail: $errno - $errstr");
		}

		fwrite($client, $data);
		$rt = stream_get_line($client, 4096, "\n"); 
		echo $rt;
		fclose($client);
